package datasource

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

var _ datasource.DataSource = &PrivateLinkEndpointServicesDataSource{}

type PrivateLinkEndpointServicesDataSource struct {
	client *client.FormationClient
}

func NewPrivateLinkEndpointServicesDataSource() datasource.DataSource {
	return &PrivateLinkEndpointServicesDataSource{}
}

type PrivateLinkEndpointServicesDataSourceModel struct {
	CloudProvider       types.String `tfsdk:"cloud_provider"`
	Region              types.String `tfsdk:"region"`
	EndpointServiceID   types.String `tfsdk:"endpoint_service_id"`
	EndpointServiceName types.String `tfsdk:"endpoint_service_name"`
	Services            types.List   `tfsdk:"services"`
	Total               types.Int64  `tfsdk:"total"`
}

func (d *PrivateLinkEndpointServicesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_private_link_endpoint_services"
}

func (d *PrivateLinkEndpointServicesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List outbound PrivateLink endpoint services registered with VeloDB Cloud.",
		Attributes: map[string]schema.Attribute{
			"cloud_provider": schema.StringAttribute{
				Description: "Cloud provider filter (e.g., aws, aliyun).",
				Optional:    true,
			},
			"region": schema.StringAttribute{
				Description: "Cloud region filter.",
				Optional:    true,
			},
			"endpoint_service_id": schema.StringAttribute{
				Description: "Exact cloud-side endpoint service ID filter.",
				Optional:    true,
			},
			"endpoint_service_name": schema.StringAttribute{
				Description: "Exact cloud-side endpoint service name filter.",
				Optional:    true,
			},
			"total": schema.Int64Attribute{
				Description: "Total number of matching endpoint services.",
				Computed:    true,
			},
			"services": schema.ListNestedAttribute{
				Description: "List of outbound PrivateLink endpoint services.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"cloud_provider":        schema.StringAttribute{Computed: true},
						"region":                schema.StringAttribute{Computed: true},
						"zone":                  schema.StringAttribute{Computed: true},
						"endpoint_service_id":   schema.StringAttribute{Computed: true},
						"endpoint_service_name": schema.StringAttribute{Computed: true},
						"provider_account_id":   schema.StringAttribute{Computed: true},
						"description":           schema.StringAttribute{Computed: true},
						"created_at":            schema.StringAttribute{Computed: true},
						"connected":             schema.BoolAttribute{Computed: true},
						"endpoints": schema.ListNestedAttribute{
							Description: "Private endpoints connected to this outbound service.",
							Computed:    true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"endpoint_id":   schema.StringAttribute{Computed: true},
									"endpoint_name": schema.StringAttribute{Computed: true},
									"domain":        schema.StringAttribute{Computed: true},
									"status":        schema.StringAttribute{Computed: true},
									"created_at":    schema.StringAttribute{Computed: true},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *PrivateLinkEndpointServicesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PrivateLinkEndpointServicesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config PrivateLinkEndpointServicesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := &client.ListPrivateLinkEndpointServicesOptions{}
	if !config.CloudProvider.IsNull() && !config.CloudProvider.IsUnknown() {
		opts.CloudProvider = config.CloudProvider.ValueString()
	}
	if !config.Region.IsNull() && !config.Region.IsUnknown() {
		opts.Region = config.Region.ValueString()
	}

	services, err := d.client.ListPrivateLinkEndpointServices(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing PrivateLink endpoint services", err.Error())
		return
	}

	services = filterPrivateLinkEndpointServices(services, config.EndpointServiceID, config.EndpointServiceName)

	serviceAttrTypes := privateLinkEndpointServiceAttrTypes()
	items := make([]attr.Value, 0, len(services))
	for _, svc := range services {
		createdAt := types.StringNull()
		if svc.CreatedAt != nil {
			createdAt = types.StringValue(svc.CreatedAt.Format(time.RFC3339))
		}

		obj, diags := types.ObjectValue(serviceAttrTypes, map[string]attr.Value{
			"cloud_provider":        stringVal(svc.CloudProvider),
			"region":                stringVal(svc.Region),
			"zone":                  stringVal(svc.Zone),
			"endpoint_service_id":   stringVal(svc.EndpointServiceID),
			"endpoint_service_name": stringVal(svc.EndpointServiceName),
			"provider_account_id":   stringVal(svc.ProviderAccountID),
			"description":           stringVal(svc.Description),
			"created_at":            createdAt,
			"connected":             types.BoolValue(svc.Connected),
			"endpoints":             privateLinkEndpointServiceEndpointsToList(svc.Endpoints, &resp.Diagnostics),
		})
		resp.Diagnostics.Append(diags...)
		items = append(items, obj)
	}

	list, diags := types.ListValue(types.ObjectType{AttrTypes: serviceAttrTypes}, items)
	resp.Diagnostics.Append(diags...)

	config.Services = list
	config.Total = types.Int64Value(int64(len(services)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func privateLinkEndpointServiceAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"cloud_provider":        types.StringType,
		"region":                types.StringType,
		"zone":                  types.StringType,
		"endpoint_service_id":   types.StringType,
		"endpoint_service_name": types.StringType,
		"provider_account_id":   types.StringType,
		"description":           types.StringType,
		"created_at":            types.StringType,
		"connected":             types.BoolType,
		"endpoints":             types.ListType{ElemType: types.ObjectType{AttrTypes: privateLinkEndpointServiceEndpointAttrTypes()}},
	}
}

func privateLinkEndpointServiceEndpointAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"endpoint_id":   types.StringType,
		"endpoint_name": types.StringType,
		"domain":        types.StringType,
		"status":        types.StringType,
		"created_at":    types.StringType,
	}
}

func privateLinkEndpointServiceEndpointsToList(endpoints []client.PrivateLinkEndpointServiceEndpoint, diags *diag.Diagnostics) types.List {
	endpointType := types.ObjectType{AttrTypes: privateLinkEndpointServiceEndpointAttrTypes()}
	if len(endpoints) == 0 {
		list, d := types.ListValue(endpointType, nil)
		diags.Append(d...)
		return list
	}

	items := make([]attr.Value, 0, len(endpoints))
	for _, ep := range endpoints {
		createdAt := types.StringNull()
		if ep.CreatedAt != nil {
			createdAt = types.StringValue(ep.CreatedAt.Format(time.RFC3339))
		}
		obj, d := types.ObjectValue(privateLinkEndpointServiceEndpointAttrTypes(), map[string]attr.Value{
			"endpoint_id":   stringVal(ep.EndpointID),
			"endpoint_name": stringVal(ep.EndpointName),
			"domain":        stringVal(ep.Domain),
			"status":        stringVal(ep.Status),
			"created_at":    createdAt,
		})
		diags.Append(d...)
		items = append(items, obj)
	}

	list, d := types.ListValue(endpointType, items)
	diags.Append(d...)
	return list
}

func filterPrivateLinkEndpointServices(services []client.PrivateLinkEndpointService, endpointServiceID, endpointServiceName types.String) []client.PrivateLinkEndpointService {
	if (endpointServiceID.IsNull() || endpointServiceID.IsUnknown()) &&
		(endpointServiceName.IsNull() || endpointServiceName.IsUnknown()) {
		return services
	}

	filtered := make([]client.PrivateLinkEndpointService, 0, len(services))
	for _, svc := range services {
		if !endpointServiceID.IsNull() && !endpointServiceID.IsUnknown() &&
			!strings.EqualFold(svc.EndpointServiceID, endpointServiceID.ValueString()) {
			continue
		}
		if !endpointServiceName.IsNull() && !endpointServiceName.IsUnknown() &&
			!strings.EqualFold(svc.EndpointServiceName, endpointServiceName.ValueString()) {
			continue
		}
		filtered = append(filtered, svc)
	}
	return filtered
}
