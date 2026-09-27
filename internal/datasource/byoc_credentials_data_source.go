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

var _ datasource.DataSource = &BYOCCredentialsDataSource{}

type BYOCCredentialsDataSource struct {
	client *client.FormationClient
}

func NewBYOCCredentialsDataSource() datasource.DataSource {
	return &BYOCCredentialsDataSource{}
}

type BYOCCredentialsDataSourceModel struct {
	CloudProvider types.String `tfsdk:"cloud_provider"`
	Region        types.String `tfsdk:"region"`
	Credentials   types.List   `tfsdk:"credentials"`
	Total         types.Int64  `tfsdk:"total"`
}

func (d *BYOCCredentialsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_byoc_credentials"
}

func (d *BYOCCredentialsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List BYOC credential configurations. Use this to discover the " +
			"credential_id needed to import a velodb_byoc_credential resource or to " +
			"reference an existing credential from a velodb_byoc_network resource.",
		Attributes: map[string]schema.Attribute{
			"cloud_provider": schema.StringAttribute{
				Description: "Cloud provider to list credential configurations for, such as `aws`.",
				Required:    true,
			},
			"region": schema.StringAttribute{
				Description: "Optional cloud region filter.",
				Optional:    true,
			},
			"total": schema.Int64Attribute{
				Description: "Total number of matching credential configurations.",
				Computed:    true,
			},
			"credentials": schema.ListNestedAttribute{
				Description: "List of credential configurations.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed:    true,
							Description: "VeloDB credential configuration ID. Use this as the import ID.",
						},
						"name":                      schema.StringAttribute{Computed: true},
						"cloud_provider":            schema.StringAttribute{Computed: true},
						"region":                    schema.StringAttribute{Computed: true},
						"bucket_name":               schema.StringAttribute{Computed: true},
						"data_credential_arn":       schema.StringAttribute{Computed: true},
						"deployment_credential_arn": schema.StringAttribute{Computed: true},
						"external_id":               schema.StringAttribute{Computed: true},
						"warehouse_count":           schema.Int64Attribute{Computed: true},
						"warehouse_ids":             schema.ListAttribute{Computed: true, ElementType: types.StringType},
						"created_at":                schema.StringAttribute{Computed: true},
						"updated_at":                schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *BYOCCredentialsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

var byocCredentialAttrTypes = map[string]attr.Type{
	"id":                        types.Int64Type,
	"name":                      types.StringType,
	"cloud_provider":            types.StringType,
	"region":                    types.StringType,
	"bucket_name":               types.StringType,
	"data_credential_arn":       types.StringType,
	"deployment_credential_arn": types.StringType,
	"external_id":               types.StringType,
	"warehouse_count":           types.Int64Type,
	"warehouse_ids":             types.ListType{ElemType: types.StringType},
	"created_at":                types.StringType,
	"updated_at":                types.StringType,
}

func (d *BYOCCredentialsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config BYOCCredentialsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	provider := config.CloudProvider.ValueString()
	region := config.Region
	list, total := renderCloudSettingList(ctx, &resp.Diagnostics, "BYOC credential configurations", byocCredentialAttrTypes,
		func(page, size int) ([]client.CloudSettingCredential, int64, error) {
			opts := &client.ListCloudSettingCredentialsOptions{Page: page, Size: size}
			if !region.IsNull() {
				opts.Region = region.ValueString()
			}
			result, err := d.client.ListCloudSettingCredentials(ctx, provider, opts)
			if err != nil {
				return nil, 0, err
			}
			return result.Data, result.Total, nil
		},
		func(ctx context.Context, diags *diag.Diagnostics, item client.CloudSettingCredential) attr.Value {
			warehouseIDs, whDiags := types.ListValueFrom(ctx, types.StringType, item.WarehouseIDs)
			diags.Append(whDiags...)
			obj, objDiags := types.ObjectValue(byocCredentialAttrTypes, map[string]attr.Value{
				"id":                        types.Int64Value(item.CredentialID),
				"name":                      types.StringValue(item.Name),
				"cloud_provider":            types.StringValue(item.CloudProvider),
				"region":                    types.StringValue(item.Region),
				"bucket_name":               stringVal(item.BucketName),
				"data_credential_arn":       stringVal(item.DataCredentialARN),
				"deployment_credential_arn": stringVal(item.DeploymentCredentialARN),
				"external_id":               stringVal(item.ExternalID),
				"warehouse_count":           types.Int64Value(int64(item.WarehouseCount)),
				"warehouse_ids":             warehouseIDs,
				"created_at":                timeVal(item.CreatedAt),
				"updated_at":                timeVal(item.UpdatedAt),
			})
			diags.Append(objDiags...)
			return obj
		},
	)
	if resp.Diagnostics.HasError() {
		return
	}

	config.Credentials = list
	config.Total = types.Int64Value(total)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
