package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

var (
	_ resource.Resource                = &WarehousePrivateEndpointResource{}
	_ resource.ResourceWithImportState = &WarehousePrivateEndpointResource{}
)

type WarehousePrivateEndpointResource struct {
	client *client.FormationClient
}

func NewWarehousePrivateEndpointResource() resource.Resource {
	return &WarehousePrivateEndpointResource{}
}

type WarehousePrivateEndpointModel struct {
	ID             types.String `tfsdk:"id"`
	WarehouseID    types.String `tfsdk:"warehouse_id"`
	EndpointID     types.String `tfsdk:"endpoint_id"`
	DNSName        types.String `tfsdk:"dns_name"`
	Description    types.String `tfsdk:"description"`
	Domain         types.String `tfsdk:"domain"`
	Status         types.String `tfsdk:"status"`
	JdbcPort       types.Int64  `tfsdk:"jdbc_port"`
	HttpPort       types.Int64  `tfsdk:"http_port"`
	StreamLoadPort types.Int64  `tfsdk:"stream_load_port"`
	AdbcPort       types.Int64  `tfsdk:"adbc_port"`
	StudioPort     types.Int64  `tfsdk:"studio_port"`
}

func (r *WarehousePrivateEndpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warehouse_private_endpoint"
}

func (r *WarehousePrivateEndpointResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages custom DNS name and description on an inbound PrivateLink endpoint connected to a VeloDB warehouse. " +
			"The endpoint itself is created cloud-side (in the customer's VPC); this resource only manages the VeloDB-side metadata.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite resource identifier (warehouse_id/endpoint_id).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"warehouse_id": schema.StringAttribute{
				Description: "Warehouse identifier.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"endpoint_id": schema.StringAttribute{
				Description: "Cloud-side PrivateLink endpoint identifier.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dns_name": schema.StringAttribute{
				Description: "Custom DNS name to associate with the inbound endpoint.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "Custom endpoint description.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain":           schema.StringAttribute{Computed: true, Description: "Endpoint host/domain returned by the connection API."},
			"status":           schema.StringAttribute{Computed: true, Description: "Endpoint status when available."},
			"jdbc_port":        schema.Int64Attribute{Computed: true, Description: "JDBC port exposed via this endpoint."},
			"http_port":        schema.Int64Attribute{Computed: true, Description: "HTTP port exposed via this endpoint."},
			"stream_load_port": schema.Int64Attribute{Computed: true, Description: "Stream Load port."},
			"adbc_port":        schema.Int64Attribute{Computed: true, Description: "Arrow Flight SQL (ADBC) port."},
			"studio_port":      schema.Int64Attribute{Computed: true, Description: "Studio port."},
		},
	}
}

func (r *WarehousePrivateEndpointResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.FormationClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.FormationClient, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *WarehousePrivateEndpointResource) applyCustom(ctx context.Context, plan *WarehousePrivateEndpointModel) error {
	apiReq := &client.RegisterWarehousePrivateEndpointRequest{
		EndpointID: plan.EndpointID.ValueString(),
	}
	setOptionalString(&apiReq.DNSName, plan.DNSName)
	setOptionalString(&apiReq.Description, plan.Description)
	return r.client.RegisterWarehousePrivateEndpoint(
		ctx,
		plan.WarehouseID.ValueString(),
		apiReq,
	)
}

func (r *WarehousePrivateEndpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WarehousePrivateEndpointModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyCustom(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Error setting private endpoint custom metadata", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.WarehouseID.ValueString() + "/" + plan.EndpointID.ValueString())
	r.readIntoState(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WarehousePrivateEndpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WarehousePrivateEndpointModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve config-only fields (dns_name/description may not be returned if empty)
	priorDNS := state.DNSName
	priorDesc := state.Description

	r.readIntoState(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.DNSName.IsNull() {
		state.DNSName = priorDNS
	}
	if state.Description.IsNull() {
		state.Description = priorDesc
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WarehousePrivateEndpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan WarehousePrivateEndpointModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyCustom(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Error updating private endpoint custom metadata", err.Error())
		return
	}

	r.readIntoState(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WarehousePrivateEndpointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WarehousePrivateEndpointModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"Warehouse private endpoint left registered",
		"The current management API exposes endpoint registration but not deregistration. Terraform removed the resource from state only.",
	)
}

func (r *WarehousePrivateEndpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: warehouse_id/endpoint_id")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("warehouse_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("endpoint_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *WarehousePrivateEndpointResource) readIntoState(ctx context.Context, state *WarehousePrivateEndpointModel, diags *diag.Diagnostics) {
	conn, err := r.client.GetWarehouseConnections(ctx, state.WarehouseID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			state.ID = types.StringNull()
			return
		}
		diags.AddError("Error reading warehouse private connection", err.Error())
		return
	}

	var found []client.PrivateConnectionEndpoint
	for _, ep := range conn.PrivateEndpoints {
		if ep.EndpointID == state.EndpointID.ValueString() {
			found = append(found, ep)
		}
	}

	if len(found) == 0 {
		// Endpoint disappeared cloud-side
		state.ID = types.StringNull()
		return
	}

	state.ID = types.StringValue(state.WarehouseID.ValueString() + "/" + state.EndpointID.ValueString())
	state.Domain = stringOrNull(firstEndpointHost(found))
	state.Status = types.StringNull()
	state.JdbcPort = endpointPort(found, "jdbc")
	state.HttpPort = endpointPort(found, "http")
	state.StreamLoadPort = endpointPort(found, "stream_load")
	state.AdbcPort = endpointPort(found, "adbc")
	state.StudioPort = endpointPort(found, "studio")
}

func firstEndpointHost(endpoints []client.PrivateConnectionEndpoint) string {
	for _, ep := range endpoints {
		if ep.Host != "" {
			return ep.Host
		}
	}
	return ""
}

func endpointPort(endpoints []client.PrivateConnectionEndpoint, protocol string) types.Int64 {
	for _, ep := range endpoints {
		if ep.Protocol == protocol && ep.Port > 0 {
			return types.Int64Value(int64(ep.Port))
		}
	}
	return types.Int64Null()
}
