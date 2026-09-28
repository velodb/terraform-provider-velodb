package resource

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
	policy := PublicAccessPolicyModel{WarehouseID: state.ID, Policy: prior[0].Policy, Rules: prior[0].Rules}
	policyService := publicAccessPolicyService{client: r.client}
	err := policyService.readIntoState(ctx, &policy, diags)
	if err != nil {
		diags.AddError("Error reading public access policy", err.Error())
		return
	}
	if diags.HasError() {
		return
	}
	rules := policy.Rules
	obj, d := types.ObjectValue(map[string]attr.Type{"policy": types.StringType, "rules": rules.Type(ctx)}, map[string]attr.Value{"policy": policy.Policy, "rules": rules})
	diags.Append(d...)
	value, d := types.ListValue(state.AccessPolicy.ElementType(ctx), []attr.Value{obj})
	diags.Append(d...)
	if !diags.HasError() {
		state.AccessPolicy = value
	}
}
