package datasource

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

var _ datasource.DataSource = &WarehouseVersionsDataSource{}

type WarehouseVersionsDataSource struct {
	client *client.FormationClient
}

func NewWarehouseVersionsDataSource() datasource.DataSource {
	return &WarehouseVersionsDataSource{}
}

type WarehouseVersionsDataSourceModel struct {
	WarehouseID types.String `tfsdk:"warehouse_id"`
	Versions    types.List   `tfsdk:"versions"`
	DefaultID   types.Int64  `tfsdk:"default_id"`
}

func (d *WarehouseVersionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warehouse_versions"
}

func (d *WarehouseVersionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List valid upgrade target versions for a VeloDB warehouse. Use a version_id from this list as core_version_id on velodb_warehouse to trigger an upgrade.",
		Attributes: map[string]schema.Attribute{
			"warehouse_id": schema.StringAttribute{
				Description: "Warehouse identifier.",
				Required:    true,
			},
			"default_id": schema.Int64Attribute{
				Description: "version_id of the default upgrade target, or 0 if none is marked default.",
				Computed:    true,
			},
			"versions": schema.ListNestedAttribute{
				Description: "All valid target versions returned by the API.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"version_id":  schema.Int64Attribute{Computed: true, Description: "Engine version ID — pass as core_version_id."},
						"version":     schema.StringAttribute{Computed: true, Description: "Human-readable version (e.g. 3.0.8)."},
						"description": schema.StringAttribute{Computed: true, Description: "Version description or release label."},
						"is_default":  schema.BoolAttribute{Computed: true, Description: "Whether this is the default upgrade target."},
					},
				},
			},
		},
	}
}

func (d *WarehouseVersionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func warehouseVersionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"version_id":  types.Int64Type,
		"version":     types.StringType,
		"description": types.StringType,
		"is_default":  types.BoolType,
	}
}

func (d *WarehouseVersionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WarehouseVersionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	versions, err := d.client.ListWarehouseVersions(ctx, data.WarehouseID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing warehouse versions", err.Error())
		return
	}

	objs := make([]attr.Value, 0, len(versions))
	var defaultID int64
	for _, v := range versions {
		obj, diags := types.ObjectValue(warehouseVersionAttrTypes(), map[string]attr.Value{
			"version_id":  types.Int64Value(v.VersionID),
			"version":     types.StringValue(v.Version),
			"description": types.StringValue(v.Description),
			"is_default":  types.BoolValue(v.IsDefault),
		})
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
		if v.IsDefault && defaultID == 0 {
			defaultID = v.VersionID
		}
	}

	list, diags := types.ListValue(types.ObjectType{AttrTypes: warehouseVersionAttrTypes()}, objs)
	resp.Diagnostics.Append(diags...)
	data.Versions = list
	data.DefaultID = types.Int64Value(defaultID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
