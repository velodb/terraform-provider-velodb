package datasource

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

// ponytail: commercial AWS principal; replace this value with the prerequisites API response when environments diverge.
const awsDeploymentAssumerRoleARN = "arn:aws:iam::757278738533:role/VeloDBDeploymentAssumer"

var _ datasource.DataSource = &BYOCPrerequisitesDataSource{}

type BYOCPrerequisitesDataSource struct {
	client *client.FormationClient
}

func NewBYOCPrerequisitesDataSource() datasource.DataSource {
	return &BYOCPrerequisitesDataSource{}
}

type BYOCPrerequisitesDataSourceModel struct {
	CloudProvider            types.String `tfsdk:"cloud_provider"`
	Region                   types.String `tfsdk:"region"`
	ExternalID               types.String `tfsdk:"external_id"`
	DeploymentAssumerRoleARN types.String `tfsdk:"deployment_assumer_role_arn"`
	EndpointServiceID        types.String `tfsdk:"endpoint_service_id"`
	EndpointServiceName      types.String `tfsdk:"endpoint_service_name"`
	MultiAZSupported         types.Bool   `tfsdk:"multi_az_supported"`
	Zones                    types.List   `tfsdk:"zones"`
}

func (d *BYOCPrerequisitesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_byoc_prerequisites"
}

func (d *BYOCPrerequisitesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Discover the organization and regional prerequisites for an AWS advanced BYOC warehouse.",
		Attributes: map[string]schema.Attribute{
			"cloud_provider": schema.StringAttribute{
				Description: "Cloud provider. Only aws is supported in this release.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("aws"),
				},
			},
			"region": schema.StringAttribute{
				Description: "AWS region to use for the BYOC warehouse.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"external_id": schema.StringAttribute{
				Description: "Organization AWS external ID for the deployment-role trust policy.",
				Computed:    true,
			},
			"deployment_assumer_role_arn": schema.StringAttribute{
				Description: "VeloDB AWS principal ARN allowed to assume the deployment role.",
				Computed:    true,
			},
			"endpoint_service_id": schema.StringAttribute{
				Description: "VeloDB PrivateLink endpoint service ID for the selected region.",
				Computed:    true,
			},
			"endpoint_service_name": schema.StringAttribute{
				Description: "VeloDB PrivateLink endpoint service name for the selected region.",
				Computed:    true,
			},
			"multi_az_supported": schema.BoolAttribute{
				Description: "Whether the selected region supports multi-AZ deployment.",
				Computed:    true,
			},
			"zones": schema.ListNestedAttribute{
				Description: "Availability zones returned for the selected region.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"zone":         schema.StringAttribute{Computed: true},
						"display_name": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *BYOCPrerequisitesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.FormationClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.FormationClient, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *BYOCPrerequisitesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data BYOCPrerequisitesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organization, err := d.client.GetOrganizationProfile(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization profile", err.Error())
		return
	}

	regions, err := d.client.ListCloudProviderRegions(ctx, data.CloudProvider.ValueString(), "BYOC")
	if err != nil {
		resp.Diagnostics.AddError("Error listing BYOC regions", err.Error())
		return
	}

	var selected *client.CloudProviderRegion
	for i := range regions {
		if regions[i].Region == data.Region.ValueString() {
			selected = &regions[i]
			break
		}
	}
	if selected == nil {
		supported := make([]string, 0, len(regions))
		for i := range regions {
			supported = append(supported, regions[i].Region)
		}
		resp.Diagnostics.AddError(
			"AWS BYOC region is not supported",
			fmt.Sprintf("Region %q is not supported for BYOC. Supported regions: %s.", data.Region.ValueString(), strings.Join(supported, ", ")),
		)
		return
	}

	zoneTypes := byocPrerequisiteZoneAttrTypes()
	zones := make([]attr.Value, 0, len(selected.Zones))
	for _, zone := range selected.Zones {
		value, diags := types.ObjectValue(zoneTypes, map[string]attr.Value{
			"zone":         types.StringValue(zone.Zone),
			"display_name": types.StringValue(zone.DisplayName),
		})
		resp.Diagnostics.Append(diags...)
		zones = append(zones, value)
	}
	zoneList, diags := types.ListValue(types.ObjectType{AttrTypes: zoneTypes}, zones)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ExternalID = types.StringValue(organization.AWSExternalID)
	data.DeploymentAssumerRoleARN = types.StringValue(awsDeploymentAssumerRoleARN)
	data.EndpointServiceID = types.StringValue(selected.EndpointServiceID)
	data.EndpointServiceName = types.StringValue(selected.EndpointServiceName)
	data.MultiAZSupported = types.BoolValue(selected.MultiAZSupported)
	data.Zones = zoneList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func byocPrerequisiteZoneAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"zone":         types.StringType,
		"display_name": types.StringType,
	}
}
