package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDeletedInitialClusterWarehouseLifecycle(t *testing.T) {
	for _, node := range []bool{false, true} {
		t.Run(fmt.Sprint("node=", node), func(t *testing.T) {
			server := mockAPIServer(t)
			defer server.Close()
			original := server.Config.Handler
			var mu sync.Mutex
			missing := false
			name := "mock-warehouse"
			missingReads, capacityWrites, deletes := 0, 0, 0
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				if r.URL.Path == "/v1/warehouses/WH-MOCK-001/clusters/CL-MOCK-001" {
					if r.Method == http.MethodPatch {
						capacityWrites++
					}
					if missing {
						missingReads++
						w.WriteHeader(404)
						writeJSONResponse(t, w, map[string]any{"code": "ClusterNotFound", "message": "deleted"})
						return
					}
				}
				if r.URL.Path == "/v1/warehouses/WH-MOCK-001" {
					if r.Method == http.MethodDelete {
						deletes++
					}
					if r.Method == http.MethodPatch {
						var body struct {
							Name string `json:"name"`
						}
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
						}
						name = body.Name
						writeJSONResponse(t, w, map[string]any{"success": true})
						return
					}
					if r.Method == http.MethodGet {
						recorded := httptest.NewRecorder()
						original.ServeHTTP(recorded, r)
						if recorded.Code == http.StatusOK {
							var body map[string]any
							if err := json.Unmarshal(recorded.Body.Bytes(), &body); err != nil {
								t.Error(err)
							}
							if data, ok := body["data"].(map[string]any); ok {
								data["name"] = name
							}
							writeJSONResponse(t, w, body)
							return
						}
						w.WriteHeader(recorded.Code)
						_, _ = w.Write(recorded.Body.Bytes())
						return
					}
				}
				original.ServeHTTP(w, r)
			})
			config := func(warehouseName string, include, resized bool) string {
				initial := ""
				if include {
					capacity := "compute_vcpu=4\ncache_gb=100"
					if resized {
						capacity = "compute_vcpu=8\ncache_gb=200"
					}
					if node {
						count := 2
						if resized {
							count = 3
						}
						capacity = fmt.Sprintf("node_vcpu=8\nnode_count=%d\nnode_cache=200", count)
					}
					initial = "initial_cluster {\nzone=\"us-east-1a\"\n" + capacity + "\n}\n"
				}
				return testProviderConfig(server) + fmt.Sprintf(`
resource "velodb_warehouse" "test" {
 name=%q
 deployment_mode="SaaS"
 cloud_provider="aws"
 region="us-east-1"
 admin_password="TestPass@123"
 %s
}`, warehouseName, initial)
			}
			resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(server), Steps: []resource.TestStep{
				{Config: config("mock-warehouse", true, false)},
				{Config: config("renamed", true, false), PreConfig: func() { mu.Lock(); missing = true; mu.Unlock() }, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("velodb_warehouse.test", "id", "WH-MOCK-001"), resource.TestCheckNoResourceAttr("velodb_warehouse.test", "initial_cluster_id"))},
				{Config: config("renamed", true, false), PlanOnly: true},
				{Config: config("renamed", true, true), PlanOnly: true, ExpectError: regexp.MustCompile("Initial cluster ID unavailable")},
				{Config: config("renamed", false, false)},
				{Config: config("renamed", false, false), PlanOnly: true},
			}})
			mu.Lock()
			defer mu.Unlock()
			if missingReads != 1 || capacityWrites != 0 || deletes != 1 {
				t.Fatalf("unexpected operations missingReads=%d capacityWrites=%d warehouseDeletes=%d", missingReads, capacityWrites, deletes)
			}
		})
	}
}
