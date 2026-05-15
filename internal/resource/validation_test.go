package resource

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

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

func autoPauseListForTest(enabled types.Bool, idleTimeout types.Int64) types.List {
	obj := types.ObjectValueMust(autoPauseAttrTypes(), map[string]attr.Value{
		"enabled":              enabled,
		"idle_timeout_minutes": idleTimeout,
	})
	return types.ListValueMust(types.ObjectType{AttrTypes: autoPauseAttrTypes()}, []attr.Value{obj})
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
