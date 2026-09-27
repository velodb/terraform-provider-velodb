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

var _ datasource.DataSource = &EncryptionKeysDataSource{}

type EncryptionKeysDataSource struct {
	client *client.FormationClient
}

func NewEncryptionKeysDataSource() datasource.DataSource {
	return &EncryptionKeysDataSource{}
}

type EncryptionKeysDataSourceModel struct {
	CloudProvider  types.String `tfsdk:"cloud_provider"`
	Region         types.String `tfsdk:"region"`
	EncryptionKeys types.List   `tfsdk:"encryption_keys"`
	Total          types.Int64  `tfsdk:"total"`
}

func (d *EncryptionKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_encryption_keys"
}

func (d *EncryptionKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List registered encryption keys. Use this to discover the " +
			"encryption_key_id needed to import a velodb_encryption_key resource or to " +
			"reference an existing key from a velodb_warehouse resource.",
		Attributes: map[string]schema.Attribute{
			"cloud_provider": schema.StringAttribute{
				Description: "Cloud provider to list encryption keys for, such as `aws`.",
				Required:    true,
			},
			"region": schema.StringAttribute{
				Description: "Optional cloud region filter.",
				Optional:    true,
			},
			"total": schema.Int64Attribute{
				Description: "Total number of matching encryption keys.",
				Computed:    true,
			},
			"encryption_keys": schema.ListNestedAttribute{
				Description: "List of encryption keys.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed:    true,
							Description: "VeloDB encryption key ID. Use this as the import ID.",
						},
						"name":            schema.StringAttribute{Computed: true},
						"cloud_provider":  schema.StringAttribute{Computed: true},
						"region":          schema.StringAttribute{Computed: true},
						"key_arn":         schema.StringAttribute{Computed: true},
						"use_tde":         schema.BoolAttribute{Computed: true},
						"use_ebs":         schema.BoolAttribute{Computed: true},
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

func (d *EncryptionKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

var encryptionKeyAttrTypes = map[string]attr.Type{
	"id":              types.Int64Type,
	"name":            types.StringType,
	"cloud_provider":  types.StringType,
	"region":          types.StringType,
	"key_arn":         types.StringType,
	"use_tde":         types.BoolType,
	"use_ebs":         types.BoolType,
	"warehouse_count": types.Int64Type,
	"warehouse_ids":   types.ListType{ElemType: types.StringType},
	"created_at":      types.StringType,
	"updated_at":      types.StringType,
}

func (d *EncryptionKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config EncryptionKeysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	provider := config.CloudProvider.ValueString()
	region := config.Region
	list, total := renderCloudSettingList(ctx, &resp.Diagnostics, "encryption keys", encryptionKeyAttrTypes,
		func(page, size int) ([]client.EncryptionKey, int64, error) {
			opts := &client.ListEncryptionKeysOptions{Page: page, Size: size}
			if !region.IsNull() {
				opts.Region = region.ValueString()
			}
			result, err := d.client.ListEncryptionKeys(ctx, provider, opts)
			if err != nil {
				return nil, 0, err
			}
			return result.Data, result.Total, nil
		},
		func(ctx context.Context, diags *diag.Diagnostics, item client.EncryptionKey) attr.Value {
			warehouseIDs, whDiags := types.ListValueFrom(ctx, types.StringType, item.WarehouseIDs)
			diags.Append(whDiags...)
			obj, objDiags := types.ObjectValue(encryptionKeyAttrTypes, map[string]attr.Value{
				"id":              types.Int64Value(item.EncryptionKeyID),
				"name":            types.StringValue(item.Name),
				"cloud_provider":  types.StringValue(item.CloudProvider),
				"region":          types.StringValue(item.Region),
				"key_arn":         stringVal(item.KeyARN),
				"use_tde":         types.BoolValue(item.UseTDE),
				"use_ebs":         types.BoolValue(item.UseEBS),
				"warehouse_count": types.Int64Value(int64(item.WarehouseCount)),
				"warehouse_ids":   warehouseIDs,
				"created_at":      stringVal(item.CreatedAt),
				"updated_at":      stringVal(item.UpdatedAt),
			})
			diags.Append(objDiags...)
			return obj
		},
	)
	if resp.Diagnostics.HasError() {
		return
	}

	config.EncryptionKeys = list
	config.Total = types.Int64Value(total)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
