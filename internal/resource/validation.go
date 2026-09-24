package resource

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

// parseBYOCImportID parses a BYOC import ID of the form "aws/<id>" into its
// cloud provider and positive numeric ID. idLabel names the ID in error
// messages (for example "credential_id" or "network_config_id").
func parseBYOCImportID(importID, idLabel string) (string, int64, error) {
	parts := strings.Split(strings.TrimSpace(importID), "/")
	if len(parts) != 2 || parts[0] != "aws" || strings.TrimSpace(parts[1]) == "" {
		return "", 0, fmt.Errorf("expected format: aws/<%s>", idLabel)
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id <= 0 {
		return "", 0, fmt.Errorf("expected format: aws/<positive %s>", idLabel)
	}
	return parts[0], id, nil
}

func validateCreateOnlyRatio(diags *diag.Diagnostics, planRatio, stateRatio types.Int64) {
	if planRatio.IsUnknown() || stateRatio.IsUnknown() || planRatio.Equal(stateRatio) {
		return
	}
	diags.AddError(
		"ratio cannot be updated",
		"VeloDB accepts ratio only when creating a cluster. Keep the existing ratio value or create a new cluster with the desired ratio.",
	)
}

func validateInitialClusterCreateOnlyRatio(ctx context.Context, diags *diag.Diagnostics, plan, state types.List) {
	planRatio := initialClusterRatio(ctx, diags, plan)
	stateRatio := initialClusterRatio(ctx, diags, state)
	if diags.HasError() {
		return
	}
	validateCreateOnlyRatio(diags, planRatio, stateRatio)
}

func initialClusterRatio(ctx context.Context, diags *diag.Diagnostics, initialCluster types.List) types.Int64 {
	if initialCluster.IsNull() {
		return types.Int64Null()
	}
	if initialCluster.IsUnknown() {
		return types.Int64Unknown()
	}

	var clusters []InitialClusterModel
	diags.Append(initialCluster.ElementsAs(ctx, &clusters, false)...)
	if diags.HasError() || len(clusters) == 0 {
		return types.Int64Null()
	}
	return clusters[0].Ratio
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

func minimumCacheGb(vcpu int64) int64 {
	implied := (vcpu / 16) * 100
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

// warehouseAccessPolicyRequest converts the create-only access_policy block into
// an API request. Returns nil when the block is absent. Allowlist rules are only
// forwarded when the policy is ALLOWLIST_ONLY.
func warehouseAccessPolicyRequest(ctx context.Context, accessPolicy types.List, diags *diag.Diagnostics) *client.WarehousePublicAccessPolicyRequest {
	if accessPolicy.IsNull() || accessPolicy.IsUnknown() {
		return nil
	}

	var policies []WarehouseAccessPolicyModel
	diags.Append(accessPolicy.ElementsAs(ctx, &policies, false)...)
	if diags.HasError() || len(policies) == 0 {
		return nil
	}

	ap := policies[0]
	return &client.WarehousePublicAccessPolicyRequest{
		PublicAccessPolicy: ap.Policy.ValueString(),
		Rules:              allowlistRulesToAPI(ctx, ap.Policy.ValueString(), ap.Rules, diags),
	}
}

// allowlistRulesToAPI converts allowlist rule objects into API rules. Rules are
// only meaningful for ALLOWLIST_ONLY, so any other policy yields no rules
// regardless of what the set contains.
func allowlistRulesToAPI(ctx context.Context, policy string, rules types.Set, diags *diag.Diagnostics) []client.WarehouseAllowlistRule {
	if policy != "ALLOWLIST_ONLY" || rules.IsNull() || rules.IsUnknown() {
		return nil
	}
	var models []AllowlistRuleModel
	diags.Append(rules.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		return nil
	}
	var out []client.WarehouseAllowlistRule
	for _, rl := range models {
		out = append(out, client.WarehouseAllowlistRule{
			CIDR:        rl.CIDR.ValueString(),
			Description: rl.Description.ValueString(),
		})
	}
	return out
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

func validateWarehouseCreation(diags *diag.Diagnostics, plan *WarehouseResourceModel, allowUnknown bool) {
	missingString := func(value types.String) bool {
		return value.IsNull() || (!allowUnknown && value.IsUnknown())
	}
	missingInt64 := func(value types.Int64) bool {
		return value.IsNull() || (!allowUnknown && value.IsUnknown())
	}

	if missingString(plan.AdminPassword) {
		diags.AddError(
			"admin_password is required for creation",
			"admin_password must be set when creating a warehouse. It can be omitted when importing an existing warehouse.",
		)
	}
	if plan.InitialCluster.IsNull() || (!allowUnknown && plan.InitialCluster.IsUnknown()) {
		diags.AddError(
			"initial_cluster is required for creation",
			"Exactly one initial_cluster block must be provided when creating a warehouse. It can be omitted when importing an existing warehouse.",
		)
	} else if !plan.InitialCluster.IsUnknown() && len(plan.InitialCluster.Elements()) != 1 {
		diags.AddError(
			"exactly one initial_cluster is required",
			"Warehouse creation accepts exactly one initial_cluster block.",
		)
	}

	if !plan.AccessPolicy.IsNull() && !plan.AccessPolicy.IsUnknown() && len(plan.AccessPolicy.Elements()) > 0 &&
		!plan.DeploymentMode.IsNull() && !plan.DeploymentMode.IsUnknown() &&
		normalizeDeploymentMode(plan.DeploymentMode.ValueString()) != "BYOC" {
		diags.AddError(
			"access_policy is only supported for BYOC warehouses",
			"The management API rejects an initial access_policy for SaaS warehouses. "+
				"Remove access_policy, or set deployment_mode to BYOC.",
		)
	}

	if plan.DeploymentMode.IsNull() || plan.DeploymentMode.IsUnknown() || normalizeDeploymentMode(plan.DeploymentMode.ValueString()) != "BYOC" {
		return
	}
	if !plan.CloudProvider.IsUnknown() && (plan.CloudProvider.IsNull() || plan.CloudProvider.ValueString() != "aws") {
		diags.AddError("AWS is required for advanced BYOC", "Set cloud_provider to aws. Other cloud providers are not supported for advanced BYOC creation.")
	}
	if missingString(plan.SetupMode) {
		diags.AddError("setup_mode is required for BYOC creation", "Set setup_mode to advanced for custom-infrastructure BYOC creation.")
	} else if !plan.SetupMode.IsUnknown() && plan.SetupMode.ValueString() != "advanced" {
		diags.AddError("Only advanced BYOC setup is supported", "Set setup_mode to advanced. Guided/template BYOC creation is not supported in this release.")
	}
	if missingInt64(plan.CredentialID) {
		diags.AddError("credential_id is required for advanced BYOC", "Set credential_id to the ID of a velodb_byoc_credential resource.")
	}
	if missingInt64(plan.NetworkConfigID) {
		diags.AddError("network_config_id is required for advanced BYOC", "Set network_config_id to the ID of a velodb_byoc_network resource.")
	}
	if !plan.VpcMode.IsNull() && (!plan.VpcMode.IsUnknown() || !allowUnknown) {
		diags.AddError("vpc_mode is not supported for advanced BYOC", "Remove vpc_mode. The registered network configuration defines the VPC for advanced BYOC.")
	}
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
