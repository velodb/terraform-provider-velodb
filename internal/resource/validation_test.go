package resource

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
