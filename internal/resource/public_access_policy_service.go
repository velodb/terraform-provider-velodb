package resource

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

// Both Terraform entry points use this service. Resource adapters only handle
// ownership, state shape, and their existing create/delete lifecycle semantics.
type publicAccessPolicyService struct{ client *client.FormationClient }

func (s publicAccessPolicyService) buildAPIRequest(ctx context.Context, plan *PublicAccessPolicyModel, diags *diag.Diagnostics) *client.WarehousePublicAccessPolicyRequest {
	return &client.WarehousePublicAccessPolicyRequest{
		PublicAccessPolicy: plan.Policy.ValueString(),
		Rules:              allowlistRulesToAPI(ctx, plan.Policy.ValueString(), plan.Rules, diags),
	}
}

func (s publicAccessPolicyService) update(ctx context.Context, plan *PublicAccessPolicyModel, diags *diag.Diagnostics) error {
	request := s.buildAPIRequest(ctx, plan, diags)
	if diags.HasError() {
		return nil
	}
	return s.client.UpdateWarehousePublicAccessPolicy(ctx, plan.WarehouseID.ValueString(), request)
}

func (s publicAccessPolicyService) readIntoState(ctx context.Context, state *PublicAccessPolicyModel, diags *diag.Diagnostics) error {
	response, err := s.client.GetWarehousePublicAccessPolicy(ctx, state.WarehouseID.ValueString())
	if err != nil {
		return err
	}
	if response.PublicAccessPolicy == "" {
		return fmt.Errorf("the API returned an empty public access policy")
	}
	apiRules := response.Allowlist
	if len(apiRules) == 0 {
		apiRules = response.Rules
	}
	rules := publicAccessRulesToSet(ctx, response.PublicAccessPolicy, apiRules, state.Rules, diags)
	if !diags.HasError() {
		state.ID = state.WarehouseID
		state.Policy = types.StringValue(response.PublicAccessPolicy)
		state.Rules = rules
	}
	return nil
}

func allowlistRuleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"cidr":        types.StringType,
		"description": types.StringType,
	}
}

func publicAccessRulesToSet(ctx context.Context, policy string, apiRules []client.WarehouseAllowlistRule, prior types.Set, diags *diag.Diagnostics) types.Set {
	ruleType := types.ObjectType{AttrTypes: allowlistRuleAttrTypes()}
	if policy != "ALLOWLIST_ONLY" {
		apiRules = nil
	}
	if len(apiRules) == 0 {
		// Preserve null versus explicitly empty, but never hide externally cleared rules.
		if prior.IsNull() {
			return types.SetNull(ruleType)
		}
		return types.SetValueMust(ruleType, []attr.Value{})
	}
	var priorModels []AllowlistRuleModel
	if !prior.IsNull() && !prior.IsUnknown() {
		diags.Append(prior.ElementsAs(ctx, &priorModels, false)...)
	}
	descriptions := map[string]types.String{}
	for _, rule := range priorModels {
		descriptions[rule.CIDR.ValueString()] = rule.Description
	}
	values := make([]attr.Value, 0, len(apiRules))
	for _, rule := range apiRules {
		description := types.StringValue(rule.Description)
		if old, ok := descriptions[rule.CIDR]; ok && old.IsNull() && rule.Description == "" {
			description = old
		}
		obj, d := types.ObjectValue(allowlistRuleAttrTypes(), map[string]attr.Value{"cidr": types.StringValue(rule.CIDR), "description": description})
		diags.Append(d...)
		values = append(values, obj)
	}
	result, d := types.SetValue(ruleType, values)
	diags.Append(d...)
	return result
}
