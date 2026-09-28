package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestWarehouseInlinePublicAccessPolicy(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()
	original := ts.Config.Handler
	var mu sync.Mutex
	remote := client.WarehousePublicAccessPolicyRequest{PublicAccessPolicy: "DENY_ALL"}
	creates, deletes, patches := 0, 0, 0
	failPatch := false
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/v1/warehouses" && r.Method == http.MethodPost {
			creates++
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read create: %v", err)
				w.WriteHeader(500)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(data))
			var body struct {
				AccessPolicy *client.WarehousePublicAccessPolicyRequest `json:"accessPolicy"`
			}
			if err := json.Unmarshal(data, &body); err != nil {
				t.Errorf("decode create: %v", err)
				w.WriteHeader(500)
				return
			}
			if body.AccessPolicy != nil {
				remote = *body.AccessPolicy
			}
		}
		if strings.HasSuffix(r.URL.Path, "/connections/public/access-policy") {
			switch r.Method {
			case http.MethodGet:
				writeJSONResponse(t, w, map[string]any{"success": true, "data": map[string]any{"publicAccessPolicy": remote.PublicAccessPolicy, "allowlist": remote.Rules}})
			case http.MethodPatch:
				if failPatch {
					w.WriteHeader(400)
					writeJSONResponse(t, w, map[string]any{"code": "InvalidArgument", "message": "mock policy failure"})
					return
				}
				var updated client.WarehousePublicAccessPolicyRequest
				if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
					t.Errorf("decode policy: %v", err)
					w.WriteHeader(400)
					return
				}
				remote = updated
				patches++
				writeJSONResponse(t, w, map[string]any{"success": true, "data": map[string]any{}})
			default:
				w.WriteHeader(405)
			}
			return
		}
		if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/warehouses/") {
			deletes++
		}
		original.ServeHTTP(w, r)
	})
	config := func(block string) string {
		return testProviderConfig(ts) + fmt.Sprintf(`
resource "velodb_warehouse" "test" {
 name = "advanced-byoc"
 deployment_mode = "BYOC"
 cloud_provider = "aws"
 region = "us-east-1"
 setup_mode = "advanced"
 credential_id = 123
 network_config_id = 456
 admin_password = "TestPass@123"
 initial_cluster {
  zone = "us-east-1a"
  compute_vcpu = 4
  cache_gb = 100
 }
 %s
}
`, block)
	}
	deny := `public_access_policy { policy = "DENY_ALL" }`
	allow := `public_access_policy { policy = "ALLOW_ALL" }`
	list := `public_access_policy {
 policy = "ALLOWLIST_ONLY"
 rules = [{cidr="203.0.113.0/24"}, {cidr="198.51.100.0/24", description="office"}]
}`
	check := func(policy string) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("velodb_warehouse.test", "id", "WH-BYOC-CREATE-001"),
			resource.TestCheckResourceAttr("velodb_warehouse.test", "public_access_policy.0.policy", policy))
	}
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts), Steps: []resource.TestStep{
		{Config: config(list), Check: check("ALLOWLIST_ONLY")},
		{Config: config(deny), Check: check("DENY_ALL")},
		{Config: config(list), Check: resource.ComposeAggregateTestCheckFunc(check("ALLOWLIST_ONLY"), resource.TestCheckResourceAttr("velodb_warehouse.test", "public_access_policy.0.rules.#", "2"))},
		{Config: config(list), PlanOnly: true},
		{Config: config(strings.Replace(list, "203.0.113.0/24", "192.0.2.0/24", 1)), Check: check("ALLOWLIST_ONLY")},
		{Config: config(allow), Check: check("ALLOW_ALL")},
		{Config: config(deny), Check: check("DENY_ALL")},
		{Config: config(list), Check: check("ALLOWLIST_ONLY")},
		// Detect an externally cleared allowlist, even when the policy mode is unchanged.
		{PreConfig: func() { mu.Lock(); remote.Rules = nil; mu.Unlock() }, Config: config(list), PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: config(list), Check: check("ALLOWLIST_ONLY")},
		{PreConfig: func() { mu.Lock(); remote.PublicAccessPolicy = "DENY_ALL"; remote.Rules = nil; mu.Unlock() }, Config: config(list), PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: config(list), Check: check("ALLOWLIST_ONLY")},

		// Removing the block must leave the policy and rules untouched.
		{Config: config("")},
		{PreConfig: func() {
			mu.Lock()
			defer mu.Unlock()
			if remote.PublicAccessPolicy != "ALLOWLIST_ONLY" || len(remote.Rules) != 2 {
				t.Fatal("removing block changed remote policy")
			}
		}, Config: config(""), PlanOnly: true},
		{Config: config(deny), Check: check("DENY_ALL")},
		{PreConfig: func() { mu.Lock(); failPatch = true; mu.Unlock() }, Config: config(allow), ExpectError: regexp.MustCompile("Error updating public access policy")},
		{PreConfig: func() { mu.Lock(); failPatch = false; mu.Unlock() }, Config: config(deny), PlanOnly: true},
	}})
	mu.Lock()
	defer mu.Unlock()
	if creates != 1 || deletes != 1 {
		t.Fatalf("warehouse must never be replaced: creates=%d deletes=%d", creates, deletes)
	}
	if patches == 0 {
		t.Fatal("no in-place policy updates occurred")
	}
}
