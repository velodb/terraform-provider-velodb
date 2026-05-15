package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func validateClusterCapacity(diags *diag.Diagnostics, label string, computeVcpu, cacheGb types.Int64) {
	if computeVcpu.IsNull() || computeVcpu.IsUnknown() || cacheGb.IsNull() || cacheGb.IsUnknown() {
		return
	}

	vcpu := computeVcpu.ValueInt64()
	cache := cacheGb.ValueInt64()

	if !validComputeVcpu(vcpu) {
		diags.AddError(
			"Invalid compute_vcpu",
			fmt.Sprintf("%s compute_vcpu must be 4, 8, 16, or a multiple of 16 above 16.", label),
		)
		return
	}

	minCache := minimumCacheGb(vcpu)
	if cache < minCache {
		diags.AddError(
			"Invalid cache_gb",
			fmt.Sprintf("%s cache_gb must be at least %d for compute_vcpu=%d.", label, minCache, vcpu),
		)
	}
	maxCache := vcpu * 100
	if cache > maxCache {
		diags.AddError(
			"Invalid cache_gb",
			fmt.Sprintf("%s cache_gb must be no more than compute_vcpu * 100 (%d).", label, maxCache),
		)
	}
}

func validateAutoPauseRequiresTimeout(ctx context.Context, diags *diag.Diagnostics, label string, autoPause types.List) {
	if autoPause.IsNull() || autoPause.IsUnknown() {
		return
	}

	var blocks []AutoPauseModel
	diags.Append(autoPause.ElementsAs(ctx, &blocks, false)...)
	if diags.HasError() {
		return
	}

	for _, block := range blocks {
		if block.Enabled.IsNull() || block.Enabled.IsUnknown() {
			continue
		}
		if block.Enabled.ValueBool() && (block.IdleTimeoutMinutes.IsNull() || block.IdleTimeoutMinutes.IsUnknown()) {
			diags.AddError(
				"idle_timeout_minutes is required when auto_pause is enabled",
				fmt.Sprintf("%s auto_pause.idle_timeout_minutes must be set when enabled is true.", label),
			)
		}
	}
}

func validComputeVcpu(v int64) bool {
	return v == 4 || v == 8 || v == 16 || (v > 16 && v%16 == 0)
}

func minimumCacheGb(vcpu int64) int64 {
	implied := vcpu * 25
	if implied < 100 {
		return 100
	}
	return implied
}

func cacheGbAfterCPUResize(oldVcpu, oldCacheGb, newVcpu int64) int64 {
	if oldVcpu <= 0 {
		return oldCacheGb
	}
	scaled := oldCacheGb * newVcpu / oldVcpu
	if minCache := minimumCacheGb(newVcpu); scaled < minCache {
		return minCache
	}
	return scaled
}

func validatePublicAccessPolicy(diags *diag.Diagnostics, policy types.String, rules types.Set) {
	if policy.IsNull() || policy.IsUnknown() || rules.IsNull() || rules.IsUnknown() {
		return
	}
	if len(rules.Elements()) == 0 || policy.ValueString() == "ALLOWLIST_ONLY" {
		return
	}
	diags.AddError(
		"rules require ALLOWLIST_ONLY",
		"Public access allowlist rules can only be set when policy is ALLOWLIST_ONLY. "+
			"Use ALLOWLIST_ONLY with rules, or remove rules for DENY_ALL/ALLOW_ALL.",
	)
}

func autoPauseToList(autoPause *client.AutoPauseConfig, diags *diag.Diagnostics) types.List {
	if autoPause == nil {
		return types.ListNull(types.ObjectType{AttrTypes: autoPauseAttrTypes()})
	}

	idleTimeout := types.Int64Null()
	if autoPause.IdleTimeoutMinutes != nil {
		idleTimeout = types.Int64Value(int64(*autoPause.IdleTimeoutMinutes))
	}

	obj, d := types.ObjectValue(autoPauseAttrTypes(), map[string]attr.Value{
		"enabled":              types.BoolValue(autoPause.Enabled),
		"idle_timeout_minutes": idleTimeout,
	})
	diags.Append(d...)

	list, d := types.ListValue(types.ObjectType{AttrTypes: autoPauseAttrTypes()}, []attr.Value{obj})
	diags.Append(d...)
	return list
}

func autoPauseAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":              types.BoolType,
		"idle_timeout_minutes": types.Int64Type,
	}
}

func normalizeDeploymentMode(mode string) string {
	if strings.EqualFold(mode, "saas") {
		return "SaaS"
	}
	if strings.EqualFold(mode, "byoc") {
		return "BYOC"
	}
	return mode
}

func rejectUnsupportedString(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse, name string) {
	var value types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &value)...)
	if !value.IsNull() && !value.IsUnknown() {
		resp.Diagnostics.AddError(
			"Unsupported "+name,
			name+" is not part of the current management API CreateWarehouseRequest.",
		)
	}
}
