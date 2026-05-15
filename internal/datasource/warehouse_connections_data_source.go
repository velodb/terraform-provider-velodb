package datasource

import (
	"context"
	"fmt"

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

type WarehouseConnectionsModel struct {
	WarehouseID         types.String                     `tfsdk:"warehouse_id"`
	EndpointServiceID   types.String                     `tfsdk:"endpoint_service_id"`
	EndpointServiceName types.String                     `tfsdk:"endpoint_service_name"`
	PublicEndpoints     []ConnectionEndpointModel        `tfsdk:"public_endpoints"`
	PrivateEndpoints    []PrivateConnectionEndpointModel `tfsdk:"private_endpoints"`
	ComputeClusters     []ConnectionClusterModel         `tfsdk:"compute_clusters"`
	ObserverGroups      []ObserverGroupModel             `tfsdk:"observer_groups"`
}

type ConnectionEndpointModel struct {
	Protocol types.String `tfsdk:"protocol"`
	Host     types.String `tfsdk:"host"`
	Port     types.Int64  `tfsdk:"port"`
	URL      types.String `tfsdk:"url"`
}

type ConnectionClusterModel struct {
	ClusterID   types.String `tfsdk:"cluster_id"`
	ClusterName types.String `tfsdk:"cluster_name"`
	HTTPPort    types.Int64  `tfsdk:"http_port"`
}

type PrivateConnectionEndpointModel struct {
	Protocol   types.String `tfsdk:"protocol"`
	Host       types.String `tfsdk:"host"`
	Port       types.Int64  `tfsdk:"port"`
	URL        types.String `tfsdk:"url"`
	EndpointID types.String `tfsdk:"endpoint_id"`
}

type ObserverGroupModel struct {
	ClusterID types.String `tfsdk:"cluster_id"`
	Name      types.String `tfsdk:"name"`
	JdbcPort  types.Int64  `tfsdk:"jdbc_port"`
}

func (d *WarehouseConnectionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warehouse_connections"
}

func (d *WarehouseConnectionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Get connection endpoints for a VeloDB warehouse.",
		Attributes: map[string]schema.Attribute{
			"warehouse_id": schema.StringAttribute{
				Description: "Warehouse identifier.",
				Required:    true,
			},
			"endpoint_service_id": schema.StringAttribute{
				Description: "PrivateLink endpoint service ID for creating cloud-side private endpoints when available.",
				Computed:    true,
			},
			"endpoint_service_name": schema.StringAttribute{
				Description: "PrivateLink endpoint service name for creating cloud-side private endpoints when available.",
				Computed:    true,
			},
			"public_endpoints": schema.ListNestedAttribute{
				Description: "Public connection endpoints (JDBC, HTTP, stream_load, ADBC, studio, MCP).",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"protocol": schema.StringAttribute{Computed: true, Description: "Protocol name (jdbc, http, stream_load, adbc, studio, mcp)."},
						"host":     schema.StringAttribute{Computed: true, Description: "Endpoint host."},
						"port":     schema.Int64Attribute{Computed: true, Description: "Endpoint port."},
						"url":      schema.StringAttribute{Computed: true, Description: "Full connection URL."},
					},
				},
			},
			"private_endpoints": schema.ListNestedAttribute{
				Description: "Private connection endpoints grouped by protocol.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"protocol":    schema.StringAttribute{Computed: true, Description: "Protocol name (jdbc, http, stream_load, adbc, studio, mcp)."},
						"host":        schema.StringAttribute{Computed: true, Description: "Endpoint host."},
						"port":        schema.Int64Attribute{Computed: true, Description: "Endpoint port."},
						"url":         schema.StringAttribute{Computed: true, Description: "Full connection URL."},
						"endpoint_id": schema.StringAttribute{Computed: true, Description: "Cloud private endpoint ID when available."},
					},
				},
			},
			"compute_clusters": schema.ListNestedAttribute{
				Description: "Compute cluster connection details.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"cluster_id":   schema.StringAttribute{Computed: true, Description: "Cluster identifier."},
						"cluster_name": schema.StringAttribute{Computed: true, Description: "Cluster name."},
						"http_port":    schema.Int64Attribute{Computed: true, Description: "HTTP port for this cluster."},
					},
				},
			},
			"observer_groups": schema.ListNestedAttribute{
				Description: "Observer group connection details.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"cluster_id": schema.StringAttribute{Computed: true, Description: "Observer cluster identifier when available."},
						"name":       schema.StringAttribute{Computed: true, Description: "Observer group name."},
						"jdbc_port":  schema.Int64Attribute{Computed: true, Description: "JDBC port for this observer group."},
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
	var config WarehouseConnectionsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	conns, err := d.client.GetWarehouseConnections(ctx, config.WarehouseID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading warehouse connections", err.Error())
		return
	}

	config.EndpointServiceID = types.StringNull()
	config.EndpointServiceName = types.StringNull()
	wh, err := d.client.GetWarehouse(ctx, config.WarehouseID.ValueString())
	if err != nil {
		resp.Diagnostics.AddWarning(
			"Unable to enrich warehouse endpoint service",
			fmt.Sprintf("Connection endpoints were read, but warehouse detail lookup failed: %s", err.Error()),
		)
	} else {
		config.EndpointServiceID = stringOrNull(wh.EndpointServiceID)
		config.EndpointServiceName = stringOrNull(wh.EndpointServiceName)
	}

	config.PublicEndpoints = make([]ConnectionEndpointModel, 0, len(conns.PublicEndpoints))
	for _, ep := range conns.PublicEndpoints {
		config.PublicEndpoints = append(config.PublicEndpoints, ConnectionEndpointModel{
			Protocol: stringOrNull(ep.Protocol),
			Host:     stringOrNull(ep.Host),
			Port:     types.Int64Value(int64(ep.Port)),
			URL:      stringOrNull(ep.URL),
		})
	}

	config.PrivateEndpoints = make([]PrivateConnectionEndpointModel, 0, len(conns.PrivateEndpoints))
	for _, ep := range conns.PrivateEndpoints {
		config.PrivateEndpoints = append(config.PrivateEndpoints, PrivateConnectionEndpointModel{
			Protocol:   stringOrNull(ep.Protocol),
			Host:       stringOrNull(ep.Host),
			Port:       types.Int64Value(int64(ep.Port)),
			URL:        stringOrNull(ep.URL),
			EndpointID: stringOrNull(ep.EndpointID),
		})
	}

	config.ComputeClusters = make([]ConnectionClusterModel, 0, len(conns.ComputeClusters))
	for _, cl := range conns.ComputeClusters {
		config.ComputeClusters = append(config.ComputeClusters, ConnectionClusterModel{
			ClusterID:   stringOrNull(cl.ClusterID),
			ClusterName: stringOrNull(cl.ClusterName),
			HTTPPort:    types.Int64Value(int64(cl.HTTPPort)),
		})
	}

	config.ObserverGroups = make([]ObserverGroupModel, 0, len(conns.ObserverGroups))
	for _, group := range conns.ObserverGroups {
		jdbcPort := types.Int64Null()
		if group.JdbcPort != nil {
			jdbcPort = types.Int64Value(int64(*group.JdbcPort))
		}
		config.ObserverGroups = append(config.ObserverGroups, ObserverGroupModel{
			ClusterID: stringOrNull(group.ClusterID),
			Name:      stringOrNull(group.Name),
			JdbcPort:  jdbcPort,
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
