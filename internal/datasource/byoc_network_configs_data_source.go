package datasource

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

var _ datasource.DataSource = &BYOCNetworkConfigsDataSource{}

type BYOCNetworkConfigsDataSource struct {
	client *client.FormationClient
}

func NewBYOCNetworkConfigsDataSource() datasource.DataSource {
	return &BYOCNetworkConfigsDataSource{}
}

type BYOCNetworkConfigsDataSourceModel struct {
	CloudProvider  types.String `tfsdk:"cloud_provider"`
	Region         types.String `tfsdk:"region"`
	NetworkConfigs types.List   `tfsdk:"network_configs"`
	Total          types.Int64  `tfsdk:"total"`
}

func (d *BYOCNetworkConfigsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_byoc_network_configs"
}

func (d *BYOCNetworkConfigsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List BYOC network configurations. Use this to discover the " +
			"network_config_id needed to import a velodb_byoc_network resource. Note the " +
			"credential_id is not returned here (the credential association lives on the " +
			"warehouse); pair a network_config_id with the credential you manage, using the " +
			"velodb_byoc_credentials data source if needed.",
		Attributes: map[string]schema.Attribute{
			"cloud_provider": schema.StringAttribute{
				Description: "Cloud provider to list network configurations for, such as `aws`.",
				Required:    true,
			},
			"region": schema.StringAttribute{
				Description: "Optional cloud region filter.",
				Optional:    true,
			},
			"total": schema.Int64Attribute{
				Description: "Total number of matching network configurations.",
				Computed:    true,
			},
			"network_configs": schema.ListNestedAttribute{
				Description: "List of network configurations.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed:    true,
							Description: "VeloDB network configuration ID. Use this as the import ID.",
						},
						"name":              schema.StringAttribute{Computed: true},
						"cloud_provider":    schema.StringAttribute{Computed: true},
						"region":            schema.StringAttribute{Computed: true},
						"vpc_id":            schema.StringAttribute{Computed: true},
						"security_group_id": schema.StringAttribute{Computed: true},
						"endpoint_id":       schema.StringAttribute{Computed: true},
						"zone_mappings": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"zone_id":   schema.StringAttribute{Computed: true},
									"subnet_id": schema.StringAttribute{Computed: true},
								},
							},
						},
						"warehouse_count": schema.Int64Attribute{Computed: true},
						"warehouse_ids":   schema.ListAttribute{Computed: true, ElementType: types.StringType},
						"created_at":      schema.StringAttribute{Computed: true},
						"updated_at":      schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *BYOCNetworkConfigsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

var byocZoneMappingAttrTypes = map[string]attr.Type{
	"zone_id":   types.StringType,
	"subnet_id": types.StringType,
}

var byocNetworkConfigAttrTypes = map[string]attr.Type{
	"id":                types.Int64Type,
	"name":              types.StringType,
	"cloud_provider":    types.StringType,
	"region":            types.StringType,
	"vpc_id":            types.StringType,
	"security_group_id": types.StringType,
	"endpoint_id":       types.StringType,
	"zone_mappings":     types.ListType{ElemType: types.ObjectType{AttrTypes: byocZoneMappingAttrTypes}},
	"warehouse_count":   types.Int64Type,
	"warehouse_ids":     types.ListType{ElemType: types.StringType},
	"created_at":        types.StringType,
	"updated_at":        types.StringType,
}

func (d *BYOCNetworkConfigsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config BYOCNetworkConfigsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	provider := config.CloudProvider.ValueString()
	region := config.Region
	list, total := renderCloudSettingList(ctx, &resp.Diagnostics, "BYOC network configurations", byocNetworkConfigAttrTypes,
		func(page, size int) ([]client.CloudSettingNetworkConfig, int64, error) {
			opts := &client.ListCloudSettingNetworkConfigsOptions{Page: page, Size: size}
			if !region.IsNull() {
				opts.Region = region.ValueString()
			}
			result, err := d.client.ListCloudSettingNetworkConfigs(ctx, provider, opts)
			if err != nil {
				return nil, 0, err
			}
			return result.Data, result.Total, nil
		},
		func(ctx context.Context, diags *diag.Diagnostics, item client.CloudSettingNetworkConfig) attr.Value {
			warehouseIDs, whDiags := types.ListValueFrom(ctx, types.StringType, item.WarehouseIDs)
			diags.Append(whDiags...)
			obj, objDiags := types.ObjectValue(byocNetworkConfigAttrTypes, map[string]attr.Value{
				"id":                types.Int64Value(item.NetworkConfigID),
				"name":              types.StringValue(item.Name),
				"cloud_provider":    types.StringValue(item.CloudProvider),
				"region":            types.StringValue(item.Region),
				"vpc_id":            stringVal(item.VPCID),
				"security_group_id": stringVal(item.SecurityGroupID),
				"endpoint_id":       stringVal(item.EndpointID),
				"zone_mappings":     flattenNetworkZoneMappings(diags, item.ZoneMappings),
				"warehouse_count":   types.Int64Value(int64(item.WarehouseCount)),
				"warehouse_ids":     warehouseIDs,
				"created_at":        timeVal(item.CreatedAt),
				"updated_at":        timeVal(item.UpdatedAt),
			})
			diags.Append(objDiags...)
			return obj
		},
	)
	if resp.Diagnostics.HasError() {
		return
	}

	config.NetworkConfigs = list
	config.Total = types.Int64Value(total)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// flattenNetworkZoneMappings converts the API zone mappings into a types.List
// matching byocZoneMappingAttrTypes.
func flattenNetworkZoneMappings(diags *diag.Diagnostics, mappings []client.CloudSettingZoneMapping) types.List {
	objType := types.ObjectType{AttrTypes: byocZoneMappingAttrTypes}
	items := make([]attr.Value, 0, len(mappings))
	for _, zm := range mappings {
		obj, objDiags := types.ObjectValue(byocZoneMappingAttrTypes, map[string]attr.Value{
			"zone_id":   types.StringValue(zm.ZoneID),
			"subnet_id": types.StringValue(zm.SubnetID),
		})
		diags.Append(objDiags...)
		items = append(items, obj)
	}
	list, listDiags := types.ListValue(objType, items)
	diags.Append(listDiags...)
	return list
}
