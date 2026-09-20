package resource

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestValidateCreateOnlyRatio(t *testing.T) {
	tests := []struct {
		name  string
		plan  types.Int64
		state types.Int64
		want  bool
	}{
		{name: "unchanged configured ratio", plan: types.Int64Value(4), state: types.Int64Value(4)},
		{name: "unchanged unspecified ratio", plan: types.Int64Null(), state: types.Int64Null()},
		{name: "changed ratio", plan: types.Int64Value(8), state: types.Int64Value(4), want: true},
		{name: "sets ratio after creation", plan: types.Int64Value(4), state: types.Int64Null(), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateCreateOnlyRatio(&diags, tt.plan, tt.state)
			if got := diags.HasError(); got != tt.want {
				t.Fatalf("HasError() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestValidateInitialClusterCreateOnlyRatio(t *testing.T) {
	tests := []struct {
		name  string
		plan  types.List
		state types.List
		want  bool
	}{
		{name: "unchanged configured ratio", plan: initialClusterListForTest(types.Int64Value(4)), state: initialClusterListForTest(types.Int64Value(4))},
		{name: "changed ratio", plan: initialClusterListForTest(types.Int64Value(8)), state: initialClusterListForTest(types.Int64Value(4)), want: true},
		{name: "sets ratio after creation", plan: initialClusterListForTest(types.Int64Value(4)), state: initialClusterListForTest(types.Int64Null()), want: true},
		{name: "removes ratio after creation", plan: initialClusterListForTest(types.Int64Null()), state: initialClusterListForTest(types.Int64Value(4)), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateInitialClusterCreateOnlyRatio(context.Background(), &diags, tt.plan, tt.state)
			if got := diags.HasError(); got != tt.want {
				t.Fatalf("HasError() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestCacheGbAfterCPUResize(t *testing.T) {
	tests := []struct {
		name       string
		oldVcpu    int64
		oldCacheGb int64
		newVcpu    int64
		want       int64
	}{
		{name: "expand from minimum", oldVcpu: 4, oldCacheGb: 100, newVcpu: 8, want: 200},
		{name: "shrink preserves ratio", oldVcpu: 8, oldCacheGb: 400, newVcpu: 4, want: 200},
		{name: "shrink applies minimum", oldVcpu: 8, oldCacheGb: 200, newVcpu: 4, want: 100},
		{name: "expand applies elastic cache minimum", oldVcpu: 32, oldCacheGb: 100, newVcpu: 64, want: 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cacheGbAfterCPUResize(tt.oldVcpu, tt.oldCacheGb, tt.newVcpu)
			if got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestMinimumCacheGb(t *testing.T) {
	tests := []struct {
		vcpu int64
		want int64
	}{
		{vcpu: 4, want: 100},
		{vcpu: 16, want: 100},
		{vcpu: 32, want: 200},
		{vcpu: 64, want: 400},
	}

	for _, tt := range tests {
		if got := minimumCacheGb(tt.vcpu); got != tt.want {
			t.Errorf("minimumCacheGb(%d) = %d, want %d", tt.vcpu, got, tt.want)
		}
	}
}

func TestValidatePublicAccessPolicy(t *testing.T) {
	rules := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("203.0.113.10/32")})

	tests := []struct {
		name      string
		policy    types.String
		rules     types.Set
		wantError bool
	}{
		{name: "allowlist accepts rules", policy: types.StringValue("ALLOWLIST_ONLY"), rules: rules},
		{name: "deny all rejects rules", policy: types.StringValue("DENY_ALL"), rules: rules, wantError: true},
		{name: "allow all rejects rules", policy: types.StringValue("ALLOW_ALL"), rules: rules, wantError: true},
		{name: "deny all without rules", policy: types.StringValue("DENY_ALL"), rules: types.SetNull(types.StringType)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validatePublicAccessPolicy(&diags, tt.policy, tt.rules)
			if tt.wantError && !diags.HasError() {
				t.Fatal("expected validation error")
			}
			if !tt.wantError && diags.HasError() {
				t.Fatalf("unexpected validation error: %v", diags)
			}
		})
	}
}

func TestValidateAutoPauseRequiresTimeout(t *testing.T) {
	tests := []struct {
		name      string
		autoPause types.List
		wantError bool
	}{
		{name: "no block", autoPause: types.ListNull(types.ObjectType{AttrTypes: autoPauseAttrTypes()})},
		{name: "disabled without timeout", autoPause: autoPauseListForTest(types.BoolValue(false), types.Int64Null())},
		{name: "enabled with timeout", autoPause: autoPauseListForTest(types.BoolValue(true), types.Int64Value(15))},
		{name: "enabled without timeout", autoPause: autoPauseListForTest(types.BoolValue(true), types.Int64Null()), wantError: true},
		{name: "enabled with unknown timeout", autoPause: autoPauseListForTest(types.BoolValue(true), types.Int64Unknown()), wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateAutoPauseRequiresTimeout(context.Background(), &diags, "cluster", tt.autoPause)
			if tt.wantError && !diags.HasError() {
				t.Fatal("expected validation error")
			}
			if !tt.wantError && diags.HasError() {
				t.Fatalf("unexpected validation error: %v", diags)
			}
		})
	}
}

func TestValidateWarehouseCreation(t *testing.T) {
	tests := []struct {
		name         string
		allowUnknown bool
		change       func(*WarehouseResourceModel)
		wantError    string
	}{
		{name: "advanced AWS BYOC", change: func(*WarehouseResourceModel) {}},
		{
			name:         "unknown registration IDs are valid while planning",
			allowUnknown: true,
			change: func(plan *WarehouseResourceModel) {
				plan.CredentialID = types.Int64Unknown()
				plan.NetworkConfigID = types.Int64Unknown()
			},
		},
		{name: "rejects non-AWS", change: func(plan *WarehouseResourceModel) { plan.CloudProvider = types.StringValue("aliyun") }, wantError: "AWS is required"},
		{name: "requires setup mode", change: func(plan *WarehouseResourceModel) { plan.SetupMode = types.StringNull() }, wantError: "setup_mode is required"},
		{name: "rejects guided setup", change: func(plan *WarehouseResourceModel) { plan.SetupMode = types.StringValue("guided") }, wantError: "Only advanced BYOC"},
		{name: "requires credential", change: func(plan *WarehouseResourceModel) { plan.CredentialID = types.Int64Null() }, wantError: "credential_id is required"},
		{name: "requires network", change: func(plan *WarehouseResourceModel) { plan.NetworkConfigID = types.Int64Null() }, wantError: "network_config_id is required"},
		{name: "rejects vpc mode", change: func(plan *WarehouseResourceModel) { plan.VpcMode = types.StringValue("existing") }, wantError: "vpc_mode is not supported"},
		{name: "requires password", change: func(plan *WarehouseResourceModel) { plan.AdminPassword = types.StringNull() }, wantError: "admin_password is required"},
		{
			name: "requires exactly one initial cluster",
			change: func(plan *WarehouseResourceModel) {
				elements := plan.InitialCluster.Elements()
				plan.InitialCluster = types.ListValueMust(plan.InitialCluster.ElementType(context.Background()), append(elements, elements[0]))
			},
			wantError: "exactly one initial_cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := validAdvancedBYOCWarehouseForTest()
			tt.change(&plan)
			var diags diag.Diagnostics
			validateWarehouseCreation(&diags, &plan, tt.allowUnknown)
			if tt.wantError == "" {
				if diags.HasError() {
					t.Fatalf("unexpected diagnostics: %v", diags)
				}
				return
			}
			for _, diagnostic := range diags.Errors() {
				if strings.Contains(diagnostic.Summary(), tt.wantError) {
					return
				}
			}
			t.Fatalf("expected diagnostic containing %q, got %v", tt.wantError, diags)
		})
	}
}

func validAdvancedBYOCWarehouseForTest() WarehouseResourceModel {
	return WarehouseResourceModel{
		DeploymentMode:  types.StringValue("BYOC"),
		CloudProvider:   types.StringValue("aws"),
		SetupMode:       types.StringValue("advanced"),
		VpcMode:         types.StringNull(),
		CredentialID:    types.Int64Value(123),
		NetworkConfigID: types.Int64Value(456),
		AdminPassword:   types.StringValue("TestPass@123"),
		InitialCluster:  initialClusterListForTest(types.Int64Null()),
	}
}

func autoPauseListForTest(enabled types.Bool, idleTimeout types.Int64) types.List {
	obj := types.ObjectValueMust(autoPauseAttrTypes(), map[string]attr.Value{
		"enabled":              enabled,
		"idle_timeout_minutes": idleTimeout,
	})
	return types.ListValueMust(types.ObjectType{AttrTypes: autoPauseAttrTypes()}, []attr.Value{obj})
}

func initialClusterListForTest(ratio types.Int64) types.List {
	autoPauseType := types.ObjectType{AttrTypes: autoPauseAttrTypes()}
	attrTypes := map[string]attr.Type{
		"zone":         types.StringType,
		"compute_vcpu": types.Int64Type,
		"ratio":        types.Int64Type,
		"cache_gb":     types.Int64Type,
		"auto_pause":   types.ListType{ElemType: autoPauseType},
	}
	obj := types.ObjectValueMust(attrTypes, map[string]attr.Value{
		"zone":         types.StringValue("us-east-1a"),
		"compute_vcpu": types.Int64Value(4),
		"ratio":        ratio,
		"cache_gb":     types.Int64Value(100),
		"auto_pause":   types.ListNull(autoPauseType),
	})
	return types.ListValueMust(types.ObjectType{AttrTypes: attrTypes}, []attr.Value{obj})
}

func TestPreserveConfiguredPublicAccessRules(t *testing.T) {
	rules := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("203.0.113.10/32")})
	state := &PublicAccessPolicyModel{
		Policy: types.StringValue("ALLOWLIST_ONLY"),
		Rules:  types.SetValueMust(types.StringType, nil),
	}

	preserveConfiguredPublicAccessRules(state, rules)

	if state.Rules.IsNull() || len(state.Rules.Elements()) != 1 {
		t.Fatalf("expected configured rules to be preserved, got %#v", state.Rules)
	}
}

func TestPublicAccessRulesToSet(t *testing.T) {
	tests := []struct {
		name       string
		policy     string
		apiRules   []client.WarehouseAllowlistRule
		wantNull   bool
		wantLength int
	}{
		{name: "deny all normalizes rules to null", policy: "DENY_ALL", wantNull: true},
		{name: "allow all normalizes rules to null", policy: "ALLOW_ALL", wantNull: true},
		{
			name:   "allowlist keeps returned rules",
			policy: "ALLOWLIST_ONLY",
			apiRules: []client.WarehouseAllowlistRule{{
				CIDR:        "203.0.113.10/32",
				Description: "terraform-e2e",
			}},
			wantLength: 1,
		},
		{name: "allowlist empty remains empty set", policy: "ALLOWLIST_ONLY", wantLength: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			got := publicAccessRulesToSet(tt.policy, tt.apiRules, &diags)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if tt.wantNull {
				if !got.IsNull() {
					t.Fatalf("expected null rules, got %#v", got)
				}
				return
			}
			if got.IsNull() {
				t.Fatalf("expected non-null rules")
			}
			if len(got.Elements()) != tt.wantLength {
				t.Fatalf("expected %d rules, got %d", tt.wantLength, len(got.Elements()))
			}
		})
	}
}
