package resource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestPublicAccessPolicyEntryPointsReadSameState(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantError  bool
		rules      int
	}{
		{"allowlist", `{"publicAccessPolicy":"ALLOWLIST_ONLY","allowlist":[{"cidr":"203.0.113.0/24"}]}`, false, 1},
		{"legacy whitelist", `{"publicAccessPolicy":"WHITELIST_ONLY","allowlist":[{"cidr":"203.0.113.0/24"}]}`, false, 1},
		{"legacy rules response fallback", `{"publicAccessPolicy":"WHITELIST_ONLY","rules":[{"cidr":"203.0.113.0/24"}]}`, false, 1},
		{"legacy empty whitelist", `{"publicAccessPolicy":"WHITELIST_ONLY","allowlist":[]}`, false, 0},
		{"rules response fallback", `{"publicAccessPolicy":"ALLOWLIST_ONLY","rules":[{"cidr":"203.0.113.0/24"}]}`, false, 1},
		{"cleared allowlist", `{"publicAccessPolicy":"ALLOWLIST_ONLY","allowlist":[]}`, false, 0},
		{"deny all ignores stale rules", `{"publicAccessPolicy":"DENY_ALL","rules":[{"cidr":"203.0.113.0/24"}]}`, false, 0},
		{"missing policy", `{}`, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, err := w.Write([]byte(`{"success":true,"data":` + tc.body + `}`)); err != nil {
					t.Errorf("write: %v", err)
				}
			}))
			defer ts.Close()
			c := client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "test", 0, time.Second)
			ctx := context.Background()
			original := warehouseAccessPolicyListForTest("ALLOWLIST_ONLY", "203.0.113.0/24")
			var models []WarehouseAccessPolicyModel
			if d := original.ElementsAs(ctx, &models, false); d.HasError() {
				t.Fatal(d)
			}
			inline := WarehouseResourceModel{ID: types.StringValue("warehouse"), AccessPolicy: original}
			standalone := PublicAccessPolicyModel{ID: inline.ID, WarehouseID: inline.ID, Policy: models[0].Policy, Rules: models[0].Rules}
			var inlineDiags, standaloneDiags diag.Diagnostics
			(&WarehouseResource{client: c}).readManagedPublicAccessPolicy(ctx, &inline, &inlineDiags)
			(&PublicAccessPolicyResource{client: c}).readIntoState(ctx, &standalone, &standaloneDiags)
			if inlineDiags.HasError() != tc.wantError || standaloneDiags.HasError() != tc.wantError {
				t.Fatalf("inline=%v standalone=%v", inlineDiags, standaloneDiags)
			}
			if d := inline.AccessPolicy.ElementsAs(ctx, &models, false); d.HasError() {
				t.Fatal(d)
			}
			if !models[0].Policy.Equal(standalone.Policy) || !models[0].Rules.Equal(standalone.Rules) {
				t.Fatal("entry points read different state")
			}
			if strings.Contains(tc.body, "WHITELIST_ONLY") && standalone.Policy.ValueString() != "ALLOWLIST_ONLY" {
				t.Fatalf("legacy policy was not normalized: %v", standalone.Policy)
			}
			if strings.Contains(tc.body, "WHITELIST_ONLY") && tc.rules > 0 && !inline.AccessPolicy.Equal(original) {
				t.Fatal("legacy response introduced policy or rule drift")
			}
			if tc.wantError {
				if !inline.AccessPolicy.Equal(original) {
					t.Fatal("failed read changed state")
				}
			} else if len(standalone.Rules.Elements()) != tc.rules {
				t.Fatalf("rules=%v", standalone.Rules)
			}
		})
	}
}
