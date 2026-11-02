package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccInitialClusterReadFailureAndSafeAdoption(t *testing.T) {
	server := mockAPIServer(t)
	defer server.Close()
	original := server.Config.Handler
	var mu sync.Mutex
	reads, creates, writes := 0, 0, 0
	cpu, cache := 8, 200
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/v1/warehouses" && r.Method == http.MethodPost {
			creates++
		}
		if r.URL.Path == "/v1/warehouses/WH-MOCK-001/clusters/CL-MOCK-001" {
			if r.Method == http.MethodGet {
				reads++
				if reads == 1 {
					w.WriteHeader(403)
					writeJSONResponse(t, w, map[string]any{"code": "Forbidden", "message": "post-create observation denied"})
					return
				}
				writeJSONResponse(t, w, map[string]any{"success": true, "data": map[string]any{"clusterId": "CL-MOCK-001", "warehouseId": "WH-MOCK-001", "clusterType": "COMPUTE", "status": "Running", "billingSummary": map[string]int{"totalCpu": cpu, "totalDiskSizeGb": cache, "nodeCount": 1}}})
				return
			}
			if r.Method == http.MethodPatch {
				writes++
				var update struct {
					CPU   *int `json:"computeVcpu"`
					Cache *int `json:"cacheGb"`
				}
				if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
					t.Error(err)
				}
				if update.CPU != nil {
					cpu = *update.CPU
				}
				if update.Cache != nil {
					cache = *update.Cache
				}
				writeJSONResponse(t, w, map[string]any{"success": true})
				return
			}
		}
		original.ServeHTTP(w, r)
	})
	config := func(cpu, cache int) string {
		return testProviderConfig(server) + fmt.Sprintf(`
resource "velodb_warehouse" "test" {
 name="mock-warehouse"
 deployment_mode="SaaS"
 cloud_provider="aws"
 region="us-east-1"
 admin_password="TestPass@123"
 initial_cluster {
  zone="us-east-1a"
  compute_vcpu=%d
  cache_gb=%d
 }
}`, cpu, cache)
	}
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(server), Steps: []resource.TestStep{
		// Creation succeeds despite a failed observation; adoption remains pending.
		// The subsequent refreshed plan discovers drift and must stop without replacement.
		{Config: config(4, 100), ExpectNonEmptyPlan: true, ExpectError: regexp.MustCompile("Initial cluster capacity adoption required"), Check: resource.TestCheckResourceAttr("velodb_warehouse.test", "initial_cluster_capacity_adopted", "false")},
		{Config: config(4, 100), PlanOnly: true, ExpectNonEmptyPlan: true, ExpectError: regexp.MustCompile("Initial cluster capacity adoption required")},
		{Config: config(8, 200), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("velodb_warehouse.test", "id", "WH-MOCK-001"), resource.TestCheckResourceAttr("velodb_warehouse.test", "initial_cluster_id", "CL-MOCK-001"), resource.TestCheckResourceAttr("velodb_warehouse.test", "initial_cluster_capacity_adopted", "true"))},
		{Config: config(8, 300)},
		{Config: config(8, 300), PlanOnly: true},
	}})
	mu.Lock()
	defer mu.Unlock()
	if creates != 1 || writes != 1 {
		t.Fatalf("expected one creation and only the intentional post-adoption resize: creates=%d writes=%d", creates, writes)
	}
}
