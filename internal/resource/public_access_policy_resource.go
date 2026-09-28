package resource

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

var (
	_ resource.Resource                   = &PublicAccessPolicyResource{}
	_ resource.ResourceWithImportState    = &PublicAccessPolicyResource{}
	_ resource.ResourceWithModifyPlan     = &PublicAccessPolicyResource{}
	_ resource.ResourceWithValidateConfig = &PublicAccessPolicyResource{}
)

type PublicAccessPolicyResource struct {
	client *client.FormationClient
}

func NewPublicAccessPolicyResource() resource.Resource {
	return &PublicAccessPolicyResource{}
}

type PublicAccessPolicyModel struct {
	ID          types.String `tfsdk:"id"`
	WarehouseID types.String `tfsdk:"warehouse_id"`
	Policy      types.String `tfsdk:"policy"`
	Rules       types.Set    `tfsdk:"rules"`
}

type AllowlistRuleModel struct {
	CIDR        types.String `tfsdk:"cidr"`
	Description types.String `tfsdk:"description"`
}

func (r *PublicAccessPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warehouse_public_access_policy"
}

func (r *PublicAccessPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the public network access policy for a VeloDB warehouse. Supports DENY_ALL, ALLOW_ALL, or ALLOWLIST_ONLY with CIDR rules.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Resource identifier (same as warehouse_id).",
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
			"policy": schema.StringAttribute{
				Description: "Public access policy: DENY_ALL, ALLOW_ALL, or ALLOWLIST_ONLY.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("DENY_ALL", "ALLOW_ALL", "ALLOWLIST_ONLY"),
				},
			},
			"rules": schema.SetNestedAttribute{
				Description: "Allowlist CIDR rules. Only valid when policy is ALLOWLIST_ONLY. Order is not significant.",
				Optional:    true,
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"cidr": schema.StringAttribute{
							Description: "CIDR block or single IP.",
							Required:    true,
						},
						"description": schema.StringAttribute{
							Description: "Optional rule description.",
							Optional:    true,
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (r *PublicAccessPolicyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var policy types.String
	var rules types.Set
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("policy"), &policy)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("rules"), &rules)...)
	validatePublicAccessPolicy(&resp.Diagnostics, policy, rules)
}

func (r *PublicAccessPolicyResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || !req.Plan.Raw.IsKnown() {
		return
	}

	var policy types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("policy"), &policy)...)
	if resp.Diagnostics.HasError() || policy.IsNull() || policy.IsUnknown() {
		return
	}

	if policy.ValueString() != "ALLOWLIST_ONLY" {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("rules"), types.SetNull(types.ObjectType{AttrTypes: allowlistRuleAttrTypes()}))...)
	}
}

func (r *PublicAccessPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *PublicAccessPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PublicAccessPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := (publicAccessPolicyService{client: r.client}).update(ctx, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Error updating public access policy", err.Error())
	}
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = plan.WarehouseID
	r.readIntoState(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PublicAccessPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PublicAccessPolicyModel
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

func (r *PublicAccessPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan PublicAccessPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := (publicAccessPolicyService{client: r.client}).update(ctx, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Error updating public access policy", err.Error())
	}
	if resp.Diagnostics.HasError() {
		return
	}

	r.readIntoState(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PublicAccessPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PublicAccessPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Reset to DENY_ALL on delete
	state.Policy = types.StringValue("DENY_ALL")
	state.Rules = types.SetNull(types.ObjectType{AttrTypes: allowlistRuleAttrTypes()})
	if err := (publicAccessPolicyService{client: r.client}).update(ctx, &state, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddWarning("Error resetting public access policy on destroy", err.Error())
	}
}

func (r *PublicAccessPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("warehouse_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *PublicAccessPolicyResource) readIntoState(ctx context.Context, state *PublicAccessPolicyModel, diags *diag.Diagnostics) {
	err := (publicAccessPolicyService{client: r.client}).readIntoState(ctx, state, diags)
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			state.ID = types.StringNull()
			return
		}
		diags.AddError("Error reading public access policy", err.Error())
		return
	}

}
