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

var _ datasource.DataSource = &WarehouseConnectionsDataSource{}

type WarehouseConnectionsDataSource struct {
	client *client.FormationClient
}

func NewWarehouseConnectionsDataSource() datasource.DataSource {
	return &WarehouseConnectionsDataSource{}
}

type WarehouseConnectionsDataSourceModel struct {
	WarehouseID       types.String `tfsdk:"warehouse_id"`
	PublicConnection  types.List   `tfsdk:"public_connection"`
	PrivateInbound    types.List   `tfsdk:"private_inbound"`
	PrivateOutbound   types.List   `tfsdk:"private_outbound_services"`
}

func (d *WarehouseConnectionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warehouse_connections"
}

func (d *WarehouseConnectionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Get combined public + private connection information for a VeloDB warehouse.",
		Attributes: map[string]schema.Attribute{
			"warehouse_id": schema.StringAttribute{
				Description: "Warehouse identifier.",
				Required:    true,
			},
			"public_connection": schema.ListNestedAttribute{
				Description: "Public connection details (list with 0 or 1 elements).",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"host":             schema.StringAttribute{Computed: true},
						"jdbc_url":         schema.StringAttribute{Computed: true},
						"http_url":         schema.StringAttribute{Computed: true},
						"jdbc_port":        schema.Int64Attribute{Computed: true},
						"http_port":        schema.Int64Attribute{Computed: true},
						"stream_load_port": schema.Int64Attribute{Computed: true},
						"adbc_port":        schema.Int64Attribute{Computed: true},
						"studio_port":      schema.Int64Attribute{Computed: true},
						"public_access_policy": schema.StringAttribute{Computed: true},
					},
				},
			},
			"private_inbound": schema.ListNestedAttribute{
				Description: "Inbound PrivateLink connection details (list with 0 or 1 elements).",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"endpoint_service_id":   schema.StringAttribute{Computed: true},
						"endpoint_service_name": schema.StringAttribute{Computed: true},
						"enabled":               schema.BoolAttribute{Computed: true},
						"provider_account_id":   schema.StringAttribute{Computed: true},
						"description":           schema.StringAttribute{Computed: true},
						"endpoints": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"endpoint_id":      schema.StringAttribute{Computed: true},
									"domain":           schema.StringAttribute{Computed: true},
									"status":           schema.StringAttribute{Computed: true},
									"dns_name":         schema.StringAttribute{Computed: true},
									"description":      schema.StringAttribute{Computed: true},
									"jdbc_port":        schema.Int64Attribute{Computed: true},
									"http_port":        schema.Int64Attribute{Computed: true},
									"stream_load_port": schema.Int64Attribute{Computed: true},
									"adbc_port":        schema.Int64Attribute{Computed: true},
									"studio_port":      schema.Int64Attribute{Computed: true},
								},
							},
						},
					},
				},
			},
			"private_outbound_services": schema.ListNestedAttribute{
				Description: "Outbound PrivateLink endpoint services visible to this warehouse.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"endpoint_service_id":   schema.StringAttribute{Computed: true},
						"endpoint_service_name": schema.StringAttribute{Computed: true},
						"cloud_provider":        schema.StringAttribute{Computed: true},
						"region":                schema.StringAttribute{Computed: true},
						"description":           schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *WarehouseConnectionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WarehouseConnectionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config WarehouseConnectionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	conns, err := d.client.GetWarehouseConnections(ctx, config.WarehouseID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading warehouse connections", err.Error())
		return
	}

	// --- public_connection ---
	pubTypes := map[string]attr.Type{
		"host":                 types.StringType,
		"jdbc_url":             types.StringType,
		"http_url":             types.StringType,
		"jdbc_port":            types.Int64Type,
		"http_port":            types.Int64Type,
		"stream_load_port":     types.Int64Type,
		"adbc_port":            types.Int64Type,
		"studio_port":          types.Int64Type,
		"public_access_policy": types.StringType,
	}
	var pubItems []attr.Value
	if conns.PublicConnection != nil {
		p := conns.PublicConnection
		obj, d1 := types.ObjectValue(pubTypes, map[string]attr.Value{
			"host":                 stringVal(p.Host),
			"jdbc_url":             stringVal(p.JdbcURL),
			"http_url":             stringVal(p.HTTPURL),
			"jdbc_port":            intPtrVal(p.JdbcPort),
			"http_port":            intPtrVal(p.HTTPPort),
			"stream_load_port":     intPtrVal(p.StreamLoadPort),
			"adbc_port":            intPtrVal(p.AdbcPort),
			"studio_port":          intPtrVal(p.StudioPort),
			"public_access_policy": stringVal(p.PublicAccessPolicy),
		})
		resp.Diagnostics.Append(d1...)
		pubItems = append(pubItems, obj)
	}
	pubList, d1 := types.ListValue(types.ObjectType{AttrTypes: pubTypes}, pubItems)
	resp.Diagnostics.Append(d1...)
	config.PublicConnection = pubList

	// --- private_inbound ---
	epTypes := map[string]attr.Type{
		"endpoint_id":      types.StringType,
		"domain":           types.StringType,
		"status":           types.StringType,
		"dns_name":         types.StringType,
		"description":      types.StringType,
		"jdbc_port":        types.Int64Type,
		"http_port":        types.Int64Type,
		"stream_load_port": types.Int64Type,
		"adbc_port":        types.Int64Type,
		"studio_port":      types.Int64Type,
	}
	inboundTypes := map[string]attr.Type{
		"endpoint_service_id":   types.StringType,
		"endpoint_service_name": types.StringType,
		"enabled":               types.BoolType,
		"provider_account_id":   types.StringType,
		"description":           types.StringType,
		"endpoints":             types.ListType{ElemType: types.ObjectType{AttrTypes: epTypes}},
	}
	var inboundItems []attr.Value
	if conns.PrivateConnection != nil && conns.PrivateConnection.Inbound != nil {
		inb := conns.PrivateConnection.Inbound
		var epItems []attr.Value
		for _, ep := range inb.Endpoints {
			obj, dd := types.ObjectValue(epTypes, map[string]attr.Value{
				"endpoint_id":      types.StringValue(ep.EndpointID),
				"domain":           stringVal(ep.Domain),
				"status":           stringVal(ep.Status),
				"dns_name":         stringVal(ep.DNSName),
				"description":      stringVal(ep.Description),
				"jdbc_port":        intPtrVal(ep.JdbcPort),
				"http_port":        intPtrVal(ep.HttpPort),
				"stream_load_port": intPtrVal(ep.StreamLoadPort),
				"adbc_port":        intPtrVal(ep.AdbcPort),
				"studio_port":      intPtrVal(ep.StudioPort),
			})
			resp.Diagnostics.Append(dd...)
			epItems = append(epItems, obj)
		}
		epList, dd := types.ListValue(types.ObjectType{AttrTypes: epTypes}, epItems)
		resp.Diagnostics.Append(dd...)

		enabled := types.BoolNull()
		if inb.Enabled != nil {
			enabled = types.BoolValue(*inb.Enabled)
		}
		obj, dd := types.ObjectValue(inboundTypes, map[string]attr.Value{
			"endpoint_service_id":   stringVal(inb.EndpointServiceID),
			"endpoint_service_name": stringVal(inb.EndpointServiceName),
			"enabled":               enabled,
			"provider_account_id":   stringVal(inb.ProviderAccountID),
			"description":           stringVal(inb.Description),
			"endpoints":             epList,
		})
		resp.Diagnostics.Append(dd...)
		inboundItems = append(inboundItems, obj)
	}
	inboundList, d2 := types.ListValue(types.ObjectType{AttrTypes: inboundTypes}, inboundItems)
	resp.Diagnostics.Append(d2...)
	config.PrivateInbound = inboundList

	// --- private_outbound_services ---
	outTypes := map[string]attr.Type{
		"endpoint_service_id":   types.StringType,
		"endpoint_service_name": types.StringType,
		"cloud_provider":        types.StringType,
		"region":                types.StringType,
		"description":           types.StringType,
	}
	var outItems []attr.Value
	if conns.PrivateConnection != nil {
		for _, svc := range conns.PrivateConnection.OutboundServices {
			obj, dd := types.ObjectValue(outTypes, map[string]attr.Value{
				"endpoint_service_id":   stringVal(svc.EndpointServiceID),
				"endpoint_service_name": stringVal(svc.EndpointServiceName),
				"cloud_provider":        stringVal(svc.CloudProvider),
				"region":                stringVal(svc.Region),
				"description":           stringVal(svc.Description),
			})
			resp.Diagnostics.Append(dd...)
			outItems = append(outItems, obj)
		}
	}
	outList, d3 := types.ListValue(types.ObjectType{AttrTypes: outTypes}, outItems)
	resp.Diagnostics.Append(d3...)
	config.PrivateOutbound = outList

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func intPtrVal(p *int) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*p))
}
