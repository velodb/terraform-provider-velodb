package provider

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccClusterCreateFailureRetainsIdentity(t *testing.T) {
	for _, scenario := range []string{"pause rejected", "final read failed", "creation failed"} {
		t.Run(scenario, func(t *testing.T) {
			var mu sync.Mutex
			creates, deletes, gets := 0, 0, 0
			deleted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				switch {
				case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/pause"):
					w.WriteHeader(409)
					writeJSONResponse(t, w, map[string]any{"code": "OperationConflict", "message": "pause rejected"})
				case req.Method == http.MethodPost:
					creates++
					writeJSONResponse(t, w, map[string]any{"success": true, "data": map[string]any{"clusterId": "c-created"}})
				case req.Method == http.MethodDelete:
					deletes++
					deleted = true
					writeJSONResponse(t, w, map[string]any{"success": true})
				default:
					gets++
					if deleted {
						w.WriteHeader(404)
						writeJSONResponse(t, w, map[string]any{"code": "ClusterNotFound"})
						return
					}
					if scenario == "final read failed" && gets == 2 {
						w.WriteHeader(403)
						writeJSONResponse(t, w, map[string]any{"code": "Forbidden", "message": "read failed"})
						return
					}
					status := "Running"
					if scenario == "creation failed" {
						status = "CreateFailed"
					}
					writeJSONResponse(t, w, map[string]any{"success": true, "data": map[string]any{
						"clusterId": "c-created", "warehouseId": "w", "name": "analytics", "clusterType": "COMPUTE", "status": status,
						"cacheType": 0, "nodeVcpu": 8, "nodeCount": 2, "nodeCache": 200, "capacityModel": "NODE_SPEC",
						"billingSummary": map[string]any{"nodeCount": 2, "totalCpu": 16, "totalDiskSizeGb": 400},
					}})
				}
			}))
			defer server.Close()
			desired := "running"
			errorPattern := "Cluster creation incomplete"
			if scenario == "pause rejected" {
				desired = "paused"
				errorPattern = "Error pausing cluster after creation"
			}
			if scenario == "final read failed" {
				errorPattern = "Error reading cluster"
			}
			config := testProviderConfig(server) + `
resource "velodb_cluster" "test" {
 warehouse_id="w"
 name="analytics"
 cluster_type="COMPUTE"
 node_vcpu=8
 node_count=2
 node_cache=200
 cache_type=0
 desired_state="` + desired + `"
}`
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(server),
				Steps:                    []resource.TestStep{{Config: config, ExpectError: regexp.MustCompile(errorPattern)}},
			})
			mu.Lock()
			defer mu.Unlock()
			if creates != 1 || deletes != 1 {
				t.Fatalf("failed create must retain its ID so Terraform cleanup can reach it: creates=%d deletes=%d", creates, deletes)
			}
		})
	}
}
