package datasource

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

var _ datasource.DataSource = &WarehousesDataSource{}

type WarehousesDataSource struct {
	client *client.FormationClient
}

func NewWarehousesDataSource() datasource.DataSource {
	return &WarehousesDataSource{}
}

type WarehousesDataSourceModel struct {
	WarehouseID    types.String `tfsdk:"warehouse_id"`
	Name           types.String `tfsdk:"name"`
	Keyword        types.String `tfsdk:"keyword"`
	CloudProvider  types.String `tfsdk:"cloud_provider"`
	Region         types.String `tfsdk:"region"`
	DeploymentMode types.String `tfsdk:"deployment_mode"`
	Warehouses     types.List   `tfsdk:"warehouses"`
	Total          types.Int64  `tfsdk:"total"`
}

func (d *WarehousesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warehouses"
}

func (d *WarehousesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List VeloDB Cloud warehouses with optional filters.",
		Attributes: map[string]schema.Attribute{
			"warehouse_id": schema.StringAttribute{
				Description: "Exact warehouse ID filter.",
				Optional:    true,
			},
			"name": schema.StringAttribute{
				Description: "Partial warehouse name filter.",
				Optional:    true,
			},
			"keyword": schema.StringAttribute{
				Description: "Legacy local fuzzy match on warehouse name or exact ID. Prefer warehouse_id or name.",
				Optional:    true,
			},
			"cloud_provider": schema.StringAttribute{
				Description: "Cloud provider filter.",
				Optional:    true,
			},
			"region": schema.StringAttribute{
				Description: "Cloud region filter.",
				Optional:    true,
			},
			"deployment_mode": schema.StringAttribute{
				Description: "Deployment mode filter (BYOC or SaaS).",
				Optional:    true,
			},
			"total": schema.Int64Attribute{
				Description: "Total number of matching warehouses.",
				Computed:    true,
			},
			"warehouses": schema.ListNestedAttribute{
				Description: "List of warehouses.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"warehouse_id":    schema.StringAttribute{Computed: true},
						"name":            schema.StringAttribute{Computed: true},
						"status":          schema.StringAttribute{Computed: true},
						"cloud_provider":  schema.StringAttribute{Computed: true},
						"region":          schema.StringAttribute{Computed: true},
						"zone":            schema.StringAttribute{Computed: true},
						"deployment_mode": schema.StringAttribute{Computed: true},
						"core_version":    schema.StringAttribute{Computed: true},
						"pay_type":        schema.StringAttribute{Computed: true},
						"endpoint_service_id": schema.StringAttribute{
							Computed:    true,
							Description: "PrivateLink endpoint service ID when available.",
						},
						"endpoint_service_name": schema.StringAttribute{
							Computed:    true,
							Description: "PrivateLink endpoint service name when available.",
						},
						"created_at":  schema.StringAttribute{Computed: true},
						"expire_time": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *WarehousesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WarehousesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config WarehousesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := &client.ListWarehousesOptions{
		Page: 1,
		Size: 100,
	}
	if !config.WarehouseID.IsNull() {
		opts.WarehouseID = config.WarehouseID.ValueString()
	}
	if !config.Name.IsNull() {
		opts.Name = config.Name.ValueString()
	}
	if !config.CloudProvider.IsNull() {
		opts.CloudProvider = config.CloudProvider.ValueString()
	}
	if !config.Region.IsNull() {
		opts.Region = config.Region.ValueString()
	}
	if !config.DeploymentMode.IsNull() {
		opts.DeploymentMode = normalizeDeploymentMode(config.DeploymentMode.ValueString())
	}

	result, err := d.client.ListWarehouses(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing warehouses", err.Error())
		return
	}

	warehouseAttrTypes := map[string]attr.Type{
		"warehouse_id":          types.StringType,
		"name":                  types.StringType,
		"status":                types.StringType,
		"cloud_provider":        types.StringType,
		"region":                types.StringType,
		"zone":                  types.StringType,
		"deployment_mode":       types.StringType,
		"core_version":          types.StringType,
		"pay_type":              types.StringType,
		"endpoint_service_id":   types.StringType,
		"endpoint_service_name": types.StringType,
		"created_at":            types.StringType,
		"expire_time":           types.StringType,
	}

	warehouses := result.Data
	if !config.Keyword.IsNull() && !config.Keyword.IsUnknown() {
		warehouses = filterWarehousesByKeyword(warehouses, config.Keyword.ValueString())
	}

	var items []attr.Value
	for _, wh := range warehouses {
		if wh.EndpointServiceID == "" || wh.EndpointServiceName == "" {
			detail, err := d.client.GetWarehouse(ctx, wh.WarehouseID)
			if err != nil {
				resp.Diagnostics.AddWarning(
					"Unable to enrich warehouse details",
					fmt.Sprintf("Warehouse %s was listed, but detail lookup failed: %s", wh.WarehouseID, err.Error()),
				)
			} else {
				wh = mergeWarehouseDetails(wh, detail)
			}
		}

		createdAt := types.StringNull()
		if wh.CreatedAt != nil {
			createdAt = types.StringValue(wh.CreatedAt.Format(time.RFC3339))
		}
		expireTime := types.StringNull()
		if wh.ExpireTime != nil {
			expireTime = types.StringValue(wh.ExpireTime.Format(time.RFC3339))
		}
		obj, diags := types.ObjectValue(warehouseAttrTypes, map[string]attr.Value{
			"warehouse_id":          types.StringValue(wh.WarehouseID),
			"name":                  types.StringValue(wh.Name),
			"status":                stringVal(wh.Status),
			"cloud_provider":        types.StringValue(wh.CloudProvider),
			"region":                types.StringValue(wh.Region),
			"zone":                  stringVal(wh.Zone),
			"deployment_mode":       stringVal(wh.DeploymentMode),
			"core_version":          stringVal(wh.CoreVersion),
			"pay_type":              stringVal(wh.PayType),
			"endpoint_service_id":   stringVal(wh.EndpointServiceID),
			"endpoint_service_name": stringVal(wh.EndpointServiceName),
			"created_at":            createdAt,
			"expire_time":           expireTime,
		})
		resp.Diagnostics.Append(diags...)
		items = append(items, obj)
	}

	list, diags := types.ListValue(types.ObjectType{AttrTypes: warehouseAttrTypes}, items)
	resp.Diagnostics.Append(diags...)

	config.Warehouses = list
	config.Total = types.Int64Value(int64(len(warehouses)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func mergeWarehouseDetails(base client.WarehouseItem, detail *client.WarehouseItem) client.WarehouseItem {
	if detail == nil {
		return base
	}
	if base.Status == "" {
		base.Status = detail.Status
	}
	if base.Zone == "" {
		base.Zone = detail.Zone
	}
	if base.DeploymentMode == "" {
		base.DeploymentMode = detail.DeploymentMode
	}
	if base.CoreVersion == "" {
		base.CoreVersion = detail.CoreVersion
	}
	if base.PayType == "" {
		base.PayType = detail.PayType
	}
	if base.EndpointServiceID == "" {
		base.EndpointServiceID = detail.EndpointServiceID
	}
	if base.EndpointServiceName == "" {
		base.EndpointServiceName = detail.EndpointServiceName
	}
	if base.CreatedAt == nil {
		base.CreatedAt = detail.CreatedAt
	}
	if base.ExpireTime == nil {
		base.ExpireTime = detail.ExpireTime
	}
	return base
}

func filterWarehousesByKeyword(warehouses []client.WarehouseItem, keyword string) []client.WarehouseItem {
	needle := strings.ToLower(strings.TrimSpace(keyword))
	if needle == "" {
		return warehouses
	}
	filtered := make([]client.WarehouseItem, 0, len(warehouses))
	for _, wh := range warehouses {
		if strings.EqualFold(wh.WarehouseID, keyword) || strings.Contains(strings.ToLower(wh.Name), needle) {
			filtered = append(filtered, wh)
		}
	}
	return filtered
}

func normalizeDeploymentMode(mode string) string {
	if strings.EqualFold(mode, "saas") {
		return "SaaS"
	}
	if strings.EqualFold(mode, "byoc") {
		return "BYOC"
	}
	return mode
}

func stringVal(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
