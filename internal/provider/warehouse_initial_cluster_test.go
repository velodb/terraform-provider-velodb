package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func warehouseImportCheck(want map[string]string) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if len(states) != 1 {
			return fmt.Errorf("expected one imported warehouse, got %d", len(states))
		}
		for name, value := range want {
			if got := states[0].Attributes[name]; got != value {
				return fmt.Errorf("imported %s=%q, want %q", name, got, value)
			}
		}
		return nil
	}
}

func TestAccWarehouseInitialClusterIdentity(t *testing.T) {
	for _, mode := range []string{"known", "unknown", "omitted"} {
		t.Run(mode, func(t *testing.T) {
			ts := mockAPIServer(t)
			defer ts.Close()
			original := ts.Config.Handler
			var mu sync.Mutex
			lists, creates, deletes := 0, 0, 0
			var identity any
			if mode == "known" {
				identity = "c-original"
			}
			ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				switch {
				case req.URL.Path == "/v1/warehouses/WH-MOCK-001/clusters":
					lists++
					writeJSONResponse(t, w, client.PageResponse[client.ClusterItem]{Success: true, Data: []client.ClusterItem{
						{ClusterID: "c-newer", WarehouseID: "WH-MOCK-001", ClusterType: "COMPUTE"},
						{ClusterID: "c-original", WarehouseID: "WH-MOCK-001", ClusterType: "COMPUTE"},
					}, Total: 2})
					return
				case req.Method == http.MethodPost && req.URL.Path == "/v1/warehouses":
					creates++
				case req.Method == http.MethodDelete && req.URL.Path == "/v1/warehouses/WH-MOCK-001":
					deletes++
				case req.Method == http.MethodGet && req.URL.Path == "/v1/warehouses/WH-MOCK-001":
					recorded := httptest.NewRecorder()
					original.ServeHTTP(recorded, req)
					var payload map[string]any
					if err := json.Unmarshal(recorded.Body.Bytes(), &payload); err != nil {
						t.Error(err)
					}
					if data, ok := payload["data"].(map[string]any); ok {
						if mode == "omitted" {
							delete(data, "initialClusterId")
						} else {
							data["initialClusterId"] = identity
						}
					}
					w.WriteHeader(recorded.Code)
					writeJSONResponse(t, w, payload)
					return
				}
				original.ServeHTTP(w, req)
			})
			config := testProviderConfig(ts) + `
resource "velodb_warehouse" "test" {
  name            = "mock-warehouse"
  deployment_mode = "SaaS"
  cloud_provider  = "aws"
  region          = "us-east-1"
}

resource "terraform_data" "initial_member" {
  input = velodb_warehouse.test.initial_cluster_id
}
`
			updated := strings.Replace(config, `name            = "mock-warehouse"`, "name = \"mock-warehouse\"\n  admin_password_version = 1", 1)
			want := ""
			check := resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckNoResourceAttr("velodb_warehouse.test", "initial_cluster_id"),
				resource.TestCheckNoResourceAttr("terraform_data.initial_member", "input"),
			)
			if mode == "known" {
				want = "c-original"
				check = resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("velodb_warehouse.test", "initial_cluster_id", want),
					resource.TestCheckResourceAttr("terraform_data.initial_member", "input", want),
				)
			}
			steps := []resource.TestStep{
				{
					Config: config, ResourceName: "velodb_warehouse.test", ImportState: true,
					ImportStateId: "WH-MOCK-001", ImportStatePersist: true,
					ImportStateCheck: warehouseImportCheck(map[string]string{"id": "WH-MOCK-001", "initial_cluster_id": want}),
				},
				{Config: config, Check: check},
				{Config: updated, Check: check},
				{Config: updated, PlanOnly: true},
			}
			if mode == "known" {
				for _, next := range []any{"c-corrected-original", nil, "c-original"} {
					nextCheck := resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckNoResourceAttr("velodb_warehouse.test", "initial_cluster_id"),
						resource.TestCheckNoResourceAttr("terraform_data.initial_member", "input"),
					)
					if id, ok := next.(string); ok {
						nextCheck = resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("velodb_warehouse.test", "initial_cluster_id", id),
							resource.TestCheckResourceAttr("terraform_data.initial_member", "input", id),
						)
					}
					steps = append(steps, resource.TestStep{
						PreConfig: func() { mu.Lock(); identity = next; mu.Unlock() },
						Config:    updated, Check: nextCheck,
					})
				}
			}
			resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts), Steps: steps})
			mu.Lock()
			defer mu.Unlock()
			if lists != 0 || creates != 0 || deletes != 1 {
				t.Fatalf("import/read/update must not create warehouses or discover a cluster: lists=%d creates=%d deletes=%d", lists, creates, deletes)
			}
		})
	}
}
