package resource

import (
	"context"
	"fmt"
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
				Description: "Cloud provider (aliyun, tencent_cloud, hwcloud, aws-cn).",
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
				Description: "Availability zone hint (provider-dependent).",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"endpoint_service_id": schema.StringAttribute{
				Description: "Cloud-side endpoint service ID. Some providers can infer it from endpoint_service_name.",
				Optional:    true,
				Computed:    true,
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
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "Optional description.",
				Optional:    true,
				Computed:    true,
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
		EndpointServiceName: plan.EndpointServiceName.ValueString(),
	}
	setOptionalString(&apiReq.Zone, plan.Zone)
	setOptionalString(&apiReq.EndpointServiceID, plan.EndpointServiceID)
	setOptionalString(&apiReq.ProviderAccountID, plan.ProviderAccountID)
	setOptionalString(&apiReq.Description, plan.Description)

	result, err := r.client.CreatePrivateLinkEndpointService(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating PrivateLink endpoint service", err.Error())
		return
	}

	plan.ID = types.StringValue(result.EndpointServiceID)
	r.readIntoState(ctx, result.EndpointServiceID, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PrivateLinkEndpointServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PrivateLinkEndpointServiceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.readIntoState(ctx, state.ID.ValueString(), &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *PrivateLinkEndpointServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state PrivateLinkEndpointServiceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !plan.Description.Equal(state.Description) {
		updateReq := &client.UpdatePrivateLinkEndpointServiceRequest{}
		setOptionalString(&updateReq.Description, plan.Description)
		if err := r.client.UpdatePrivateLinkEndpointService(ctx, state.ID.ValueString(), updateReq); err != nil {
			resp.Diagnostics.AddError("Error updating PrivateLink endpoint service", err.Error())
			return
		}
	}

	r.readIntoState(ctx, state.ID.ValueString(), &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *PrivateLinkEndpointServiceResource) readIntoState(ctx context.Context, id string, state *PrivateLinkEndpointServiceModel, diags *diag.Diagnostics) {
	svc, err := r.client.GetPrivateLinkEndpointService(ctx, id)
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			state.ID = types.StringNull()
			return
		}
		diags.AddError("Error reading PrivateLink endpoint service", err.Error())
		return
	}

	state.ID = types.StringValue(svc.EndpointServiceID)
	state.CloudProvider = stringOrNull(svc.CloudProvider)
	state.Region = stringOrNull(svc.Region)
	state.Zone = stringOrNull(svc.Zone)
	state.EndpointServiceID = stringOrNull(svc.EndpointServiceID)
	state.EndpointServiceName = stringOrNull(svc.EndpointServiceName)
	state.ProviderAccountID = stringOrNull(svc.ProviderAccountID)
	state.Description = stringOrNull(svc.Description)
	state.Connected = types.BoolValue(svc.Connected)
	if svc.CreatedAt != nil {
		state.CreatedAt = types.StringValue(svc.CreatedAt.Format(time.RFC3339))
	} else {
		state.CreatedAt = types.StringNull()
	}
}
