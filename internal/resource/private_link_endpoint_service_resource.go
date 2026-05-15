package resource

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	_ resource.Resource                = &PrivateLinkEndpointServiceResource{}
	_ resource.ResourceWithImportState = &PrivateLinkEndpointServiceResource{}
)

type PrivateLinkEndpointServiceResource struct {
	client *client.FormationClient
}

func NewPrivateLinkEndpointServiceResource() resource.Resource {
	return &PrivateLinkEndpointServiceResource{}
}

type PrivateLinkEndpointServiceModel struct {
	ID                  types.String `tfsdk:"id"`
	CloudProvider       types.String `tfsdk:"cloud_provider"`
	Region              types.String `tfsdk:"region"`
	Zone                types.String `tfsdk:"zone"`
	EndpointServiceID   types.String `tfsdk:"endpoint_service_id"`
	EndpointServiceName types.String `tfsdk:"endpoint_service_name"`
	ProviderAccountID   types.String `tfsdk:"provider_account_id"`
	Description         types.String `tfsdk:"description"`
	CreatedAt           types.String `tfsdk:"created_at"`
	Connected           types.Bool   `tfsdk:"connected"`
}

func (r *PrivateLinkEndpointServiceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_private_link_endpoint_service"
}

func (r *PrivateLinkEndpointServiceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Registers a PrivateLink endpoint service with VeloDB Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Endpoint service identifier.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cloud_provider": schema.StringAttribute{
				Description: "Cloud provider (aws, aliyun, tencent_cloud, hwcloud, aws-cn).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region": schema.StringAttribute{
				Description: "Cloud region.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"zone": schema.StringAttribute{
				Description: "Availability zone associated with the endpoint service when known.",
				Computed:    true,
			},
			"endpoint_service_id": schema.StringAttribute{
				Description: "Cloud-side endpoint service ID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"endpoint_service_name": schema.StringAttribute{
				Description: "Cloud-side endpoint service name.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"provider_account_id": schema.StringAttribute{
				Description: "Cloud account ID used for the endpoint service registration.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Optional description.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Creation time in RFC 3339 format.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"connected": schema.BoolAttribute{
				Description: "Whether the endpoint service is currently connected.",
				Computed:    true,
			},
		},
	}
}

func (r *PrivateLinkEndpointServiceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *PrivateLinkEndpointServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PrivateLinkEndpointServiceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := &client.CreatePrivateLinkEndpointServiceRequest{
		CloudProvider:       plan.CloudProvider.ValueString(),
		Region:              plan.Region.ValueString(),
		EndpointServiceID:   plan.EndpointServiceID.ValueString(),
		EndpointServiceName: plan.EndpointServiceName.ValueString(),
	}
	setOptionalString(&apiReq.Description, plan.Description)

	result, err := r.client.CreatePrivateLinkEndpointService(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating PrivateLink endpoint service", err.Error())
		return
	}

	plan.ID = types.StringValue(result.EndpointServiceID)
	r.readIntoState(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PrivateLinkEndpointServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PrivateLinkEndpointServiceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.readIntoState(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *PrivateLinkEndpointServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"PrivateLink endpoint service cannot be updated",
		"The current management API only supports registering and deleting endpoint services. Change the configuration in a way that replaces the resource.",
	)
}

func (r *PrivateLinkEndpointServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PrivateLinkEndpointServiceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeletePrivateLinkEndpointService(ctx, state.ID.ValueString()); err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Error deleting PrivateLink endpoint service", err.Error())
	}
}

func (r *PrivateLinkEndpointServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: cloud_provider/region/endpoint_service_id")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cloud_provider"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("region"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("endpoint_service_id"), parts[2])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[2])...)
}

func (r *PrivateLinkEndpointServiceResource) readIntoState(ctx context.Context, state *PrivateLinkEndpointServiceModel, diags *diag.Diagnostics) {
	services, err := r.client.ListPrivateLinkEndpointServices(ctx, &client.ListPrivateLinkEndpointServicesOptions{
		CloudProvider: state.CloudProvider.ValueString(),
		Region:        state.Region.ValueString(),
	})
	if err != nil {
		diags.AddError("Error reading PrivateLink endpoint service", err.Error())
		return
	}
	var svc *client.PrivateLinkEndpointService
	for i := range services {
		if services[i].EndpointServiceID == state.EndpointServiceID.ValueString() || services[i].EndpointServiceID == state.ID.ValueString() {
			svc = &services[i]
			break
		}
	}
	if svc == nil {
		state.ID = types.StringNull()
		return
	}

	state.ID = types.StringValue(svc.EndpointServiceID)
	state.CloudProvider = stringOrNull(svc.CloudProvider)
	state.Region = stringOrNull(svc.Region)
	state.Zone = stringOrNull(svc.Zone)
	state.EndpointServiceID = stringOrNull(svc.EndpointServiceID)
	state.EndpointServiceName = stringOrNull(svc.EndpointServiceName)
	state.ProviderAccountID = stringOrNull(svc.ProviderAccountID)
	if svc.Description != "" || state.Description.IsNull() || state.Description.IsUnknown() {
		state.Description = stringOrNull(svc.Description)
	}
	state.Connected = types.BoolValue(svc.Connected)
	if svc.CreatedAt != nil {
		state.CreatedAt = types.StringValue(svc.CreatedAt.Format(time.RFC3339))
	} else {
		state.CreatedAt = types.StringNull()
	}
}
