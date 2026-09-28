package resource

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func (r *WarehouseResource) readManagedPublicAccessPolicy(ctx context.Context, state *WarehouseResourceModel, diags *diag.Diagnostics) {
	// Omitted blocks (including imports) do not claim ownership of the policy.
	if state.AccessPolicy.IsNull() || state.AccessPolicy.IsUnknown() || len(state.AccessPolicy.Elements()) == 0 {
		return
	}
	var prior []WarehouseAccessPolicyModel
	diags.Append(state.AccessPolicy.ElementsAs(ctx, &prior, false)...)
	if diags.HasError() {
		return
	}
	policy, err := r.client.GetWarehousePublicAccessPolicy(ctx, state.ID.ValueString())
	if err != nil {
		diags.AddError("Error reading public access policy", err.Error())
		return
	}
	if policy.PublicAccessPolicy == "" {
		diags.AddError("Error reading public access policy", "The API returned an empty public access policy.")
		return
	}
	apiRules := policy.Allowlist
	if len(apiRules) == 0 {
		apiRules = policy.Rules
	}
	rules := warehousePublicAccessRules(ctx, policy.PublicAccessPolicy, apiRules, prior[0].Rules, diags)
	obj, d := types.ObjectValue(map[string]attr.Type{"policy": types.StringType, "rules": rules.Type(ctx)}, map[string]attr.Value{"policy": types.StringValue(policy.PublicAccessPolicy), "rules": rules})
	diags.Append(d...)
	value, d := types.ListValue(state.AccessPolicy.ElementType(ctx), []attr.Value{obj})
	diags.Append(d...)
	if !diags.HasError() {
		state.AccessPolicy = value
	}
}

func warehousePublicAccessRules(ctx context.Context, policy string, apiRules []client.WarehouseAllowlistRule, prior types.Set, diags *diag.Diagnostics) types.Set {
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
