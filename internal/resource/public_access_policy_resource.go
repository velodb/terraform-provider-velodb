package resource

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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

func (r *PublicAccessPolicyResource) buildAPIRequest(ctx context.Context, plan *PublicAccessPolicyModel, diags *diag.Diagnostics) *client.WarehousePublicAccessPolicyRequest {
	apiReq := &client.WarehousePublicAccessPolicyRequest{
		PublicAccessPolicy: plan.Policy.ValueString(),
	}
	if plan.Policy.ValueString() == "ALLOWLIST_ONLY" && !plan.Rules.IsNull() && !plan.Rules.IsUnknown() {
		var rules []AllowlistRuleModel
		diags.Append(plan.Rules.ElementsAs(ctx, &rules, false)...)
		for _, rl := range rules {
			apiReq.Rules = append(apiReq.Rules, client.WarehouseAllowlistRule{
				CIDR:        rl.CIDR.ValueString(),
				Description: rl.Description.ValueString(),
			})
		}
	}
	return apiReq
}

func (r *PublicAccessPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PublicAccessPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := r.buildAPIRequest(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateWarehousePublicAccessPolicy(ctx, plan.WarehouseID.ValueString(), apiReq); err != nil {
		resp.Diagnostics.AddError("Error setting public access policy", err.Error())
		return
	}

	plan.ID = plan.WarehouseID
	priorRules := plan.Rules
	r.readIntoState(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	preserveConfiguredPublicAccessRules(&plan, priorRules)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PublicAccessPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PublicAccessPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	priorRules := state.Rules
	r.readIntoState(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	preserveConfiguredPublicAccessRules(&state, priorRules)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *PublicAccessPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan PublicAccessPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := r.buildAPIRequest(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateWarehousePublicAccessPolicy(ctx, plan.WarehouseID.ValueString(), apiReq); err != nil {
		resp.Diagnostics.AddError("Error updating public access policy", err.Error())
		return
	}

	priorRules := plan.Rules
	r.readIntoState(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	preserveConfiguredPublicAccessRules(&plan, priorRules)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PublicAccessPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PublicAccessPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Reset to DENY_ALL on delete
	apiReq := &client.WarehousePublicAccessPolicyRequest{PublicAccessPolicy: "DENY_ALL"}
	if err := r.client.UpdateWarehousePublicAccessPolicy(ctx, state.WarehouseID.ValueString(), apiReq); err != nil {
		resp.Diagnostics.AddWarning("Error resetting public access policy on destroy", err.Error())
	}
}

func (r *PublicAccessPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("warehouse_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func allowlistRuleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"cidr":        types.StringType,
		"description": types.StringType,
	}
}

func (r *PublicAccessPolicyResource) readIntoState(ctx context.Context, state *PublicAccessPolicyModel, diags *diag.Diagnostics) {
	policy, err := r.client.GetWarehousePublicAccessPolicy(ctx, state.WarehouseID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			state.ID = types.StringNull()
			return
		}
		diags.AddError("Error reading public access policy", err.Error())
		return
	}

	state.ID = state.WarehouseID
	if policy.PublicAccessPolicy != "" {
		state.Policy = types.StringValue(policy.PublicAccessPolicy)
	}

	rules := policy.Allowlist
	if len(rules) == 0 {
		rules = policy.Rules
	}
	state.Rules = publicAccessRulesToSet(state.Policy.ValueString(), rules, diags)
}

func publicAccessRulesToSet(policy string, apiRules []client.WarehouseAllowlistRule, diags *diag.Diagnostics) types.Set {
	ruleType := types.ObjectType{AttrTypes: allowlistRuleAttrTypes()}
	if policy != "ALLOWLIST_ONLY" {
		return types.SetNull(ruleType)
	}

	var rules []attr.Value
	for _, rl := range apiRules {
		obj, d := types.ObjectValue(allowlistRuleAttrTypes(), map[string]attr.Value{
			"cidr":        types.StringValue(rl.CIDR),
			"description": types.StringValue(rl.Description),
		})
		diags.Append(d...)
		rules = append(rules, obj)
	}
	list, d := types.SetValue(ruleType, rules)
	diags.Append(d...)
	return list
}

func preserveConfiguredPublicAccessRules(state *PublicAccessPolicyModel, priorRules types.Set) {
	if state.Policy.IsNull() || state.Policy.IsUnknown() || state.Policy.ValueString() != "ALLOWLIST_ONLY" {
		return
	}
	if priorRules.IsNull() || priorRules.IsUnknown() || len(priorRules.Elements()) == 0 {
		return
	}
	if !state.Rules.IsNull() && !state.Rules.IsUnknown() && len(state.Rules.Elements()) > 0 {
		return
	}
	state.Rules = priorRules
}
