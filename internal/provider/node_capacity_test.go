package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccNodeCapacityLifecycle(t *testing.T) {
	for _, tc := range []struct{ migrate, local bool }{{}, {migrate: true}, {local: true}} {
		migrate, local := tc.migrate, tc.local
		t.Run(fmt.Sprint("legacy migration=", migrate, " local=", local), func(t *testing.T) {
			var mu sync.Mutex
			cpu, count, cache := 8, 3, 200
			model := "NODE_SPEC"
			cacheType := 0
			if local {
				cacheType = 1
				cache = 474
			}
			packageCache := map[int]int{8: 474, 16: 777, 32: 1555}
			if migrate {
				model = ""
			}
			deleted := false
			patches := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPost {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if migrate {
						if body["computeVcpu"] != float64(24) || body["cacheGb"] != float64(600) {
							t.Errorf("legacy create: %v", body)
						}
					} else {
						if body["nodeVcpu"] != float64(8) || body["nodeCount"] != float64(3) || !local && body["nodeCache"] != float64(200) {
							t.Errorf("node create: %v", body)
						}
						if local && (body["cacheType"] != float64(1) || body["nodeCache"] != nil) {
							t.Errorf("local create must omit cache: %v", body)
						}
						if _, ok := body["computeVcpu"]; ok {
							t.Errorf("legacy fields leaked: %v", body)
						}
					}
					writeJSONResponse(t, w, map[string]any{"success": true, "data": map[string]any{"clusterId": "c-nodes"}})
					return
				}
				if r.Method == http.MethodPatch {
					if r.Header.Get("RequestId") == "" {
						t.Error("missing idempotency key")
					}
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if local && body["nodeCache"] != nil {
						t.Errorf("local update must omit cache: %v", body)
					}
					for _, field := range []string{"computeVcpu", "cacheGb", "ratio", "cacheType"} {
						if _, ok := body[field]; ok {
							t.Errorf("unexpected field %s: %v", field, body)
						}
					}
					if v, ok := body["nodeVcpu"].(float64); ok {
						cpu = int(v)
						if local {
							cache = packageCache[cpu]
						}
					}
					if v, ok := body["nodeCount"].(float64); ok {
						count = int(v)
					}
					if v, ok := body["nodeCache"].(float64); ok {
						cache = int(v)
					}
					model = "NODE_SPEC"
					patches++
					writeJSONResponse(t, w, map[string]any{"success": true, "data": map[string]any{}})
					return
				}
				if r.Method == http.MethodDelete {
					deleted = true
					writeJSONResponse(t, w, map[string]any{"success": true})
					return
				}
				if deleted {
					w.WriteHeader(404)
					writeJSONResponse(t, w, map[string]any{"code": "ClusterNotFound"})
					return
				}
				data := map[string]any{"clusterId": "c-nodes", "warehouseId": "w-nodes", "name": "analytics", "clusterType": "COMPUTE", "zone": "us-east-1a", "status": "Running", "cacheType": cacheType, "nodeVcpu": cpu, "nodeCount": count, "nodeCache": cache, "billingModel": "on_demand", "onDemandNodeCount": count, "capacityModel": model, "billingSummary": map[string]any{"nodeCount": count, "totalCpu": cpu * count, "totalDiskSizeGb": cache * count}, "billingPools": map[string]any{"onDemand": map[string]any{"nodeCount": count, "cpu": cpu * count, "diskSizeGb": cache * count}}}
				writeJSONResponse(t, w, map[string]any{"success": true, "data": data})
			}))
			defer server.Close()
			config := func(capacity string) string {
				return testProviderConfig(server) + `resource "velodb_cluster" "test" {
 warehouse_id="w-nodes"
 name="analytics"
 cluster_type="COMPUTE"
 ` + capacity + "\n}"
			}
			node := func(cpu, count, cache int) string {
				if local {
					return config(fmt.Sprintf("cache_type=1\nnode_vcpu=%d\nnode_count=%d", cpu, count))
				}
				return config(fmt.Sprintf("node_vcpu=%d\nnode_count=%d\nnode_cache=%d", cpu, count, cache))
			}
			diskAfterCount := "1600"
			if local {
				diskAfterCount = "3108"
			}
			steps := []resource.TestStep{}
			if migrate {
				steps = append(steps, resource.TestStep{Config: config("compute_vcpu=24\ncache_gb=600")})
			}
			steps = append(steps,
				resource.TestStep{Config: node(8, 3, 200), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("velodb_cluster.test", "capacity_model", "NODE_SPEC"), resource.TestCheckResourceAttr("velodb_cluster.test", "node_count", "3"))},
				resource.TestStep{Config: node(8, 3, 200), PlanOnly: true},
				resource.TestStep{Config: node(16, 3, 400), Check: resource.TestCheckResourceAttr("velodb_cluster.test", "total_cpu", "48")},
				resource.TestStep{Config: node(16, 4, 400), Check: resource.TestCheckResourceAttr("velodb_cluster.test", "total_disk_gb", diskAfterCount)},
				resource.TestStep{Config: node(16, 4, 400), PlanOnly: true},
				resource.TestStep{Config: node(16, 4, 400), PreConfig: func() {
					mu.Lock()
					defer mu.Unlock()
					cpu = 8
					cache = 200
					if local {
						cache = 474
					}
				}, Check: resource.TestCheckResourceAttr("velodb_cluster.test", "node_vcpu", "16")},
				resource.TestStep{ResourceName: "velodb_cluster.test", ImportState: true, ImportStateId: "w-nodes/c-nodes", ImportStateVerify: true, ImportStateVerifyIgnore: []string{"timeouts"}},
				resource.TestStep{Config: node(32, 5, 800), PlanOnly: true, ExpectError: regexp.MustCompile("Separate node count and size changes")},
				resource.TestStep{Config: node(16, 4, 400) + "\n", PlanOnly: true},
				resource.TestStep{Config: config("cache_type=" + fmt.Sprint(1-cacheType) + "\nnode_vcpu=16\nnode_count=4" + map[bool]string{true: "\nnode_cache=400", false: ""}[local]), PlanOnly: true, ExpectError: regexp.MustCompile("cache_type is creation-only")},
			)
			resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(server), Steps: steps})
			mu.Lock()
			defer mu.Unlock()
			want := 3
			if migrate {
				want = 4
			}
			if patches != want {
				t.Errorf("PATCH calls=%d, want %d", patches, want)
			}
		})
	}
}

func TestAccWarehouseNodeInitialCluster(t *testing.T) {
	for _, local := range []bool{false, true} {
		t.Run(fmt.Sprint("local=", local), func(t *testing.T) {
			ts := mockAPIServer(t)
			defer ts.Close()
			original := ts.Config.Handler
			ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.URL.Path == "/v1/warehouses" {
					raw, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					r.Body = io.NopCloser(bytes.NewReader(raw))
					var body struct {
						Initial map[string]any `json:"initialCluster"`
					}
					if err := json.Unmarshal(raw, &body); err != nil {
						t.Error(err)
					}
					if body.Initial["nodeVcpu"] != float64(8) || body.Initial["nodeCount"] != float64(2) {
						t.Errorf("initial node request: %s", raw)
					}
					for _, key := range []string{"computeVcpu", "cacheGb"} {
						if body.Initial[key] != nil {
							t.Errorf("legacy input leaked: %s", raw)
						}
					}
					if local {
						if body.Initial["cacheType"] != float64(1) || body.Initial["nodeCache"] != nil {
							t.Errorf("local request: %s", raw)
						}
					} else if body.Initial["nodeCache"] != float64(200) {
						t.Errorf("EBS request: %s", raw)
					}
				}
				original.ServeHTTP(w, r)
			})
			capacity := "node_vcpu=8\nnode_count=2\nnode_cache=200"
			if local {
				capacity = "cache_type=1\nnode_vcpu=8\nnode_count=2"
			}
			config := testProviderConfig(ts) + `resource "velodb_warehouse" "test" {
 name="mock-warehouse"
 deployment_mode="SaaS"
 cloud_provider="aws"
 region="us-east-1"
 admin_password="TestPass@123"
 initial_cluster {
 zone="us-east-1a"
 ` + capacity + `
 }
}`
			resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts), Steps: []resource.TestStep{
				{Config: config, Check: resource.TestCheckResourceAttr("velodb_warehouse.test", "initial_cluster.0.node_count", "2")},
				{Config: config, PlanOnly: true},
				{Config: strings.Replace(config, "node_count=2", "node_count=3", 1), PlanOnly: true, ExpectError: regexp.MustCompile("initial_cluster is creation-only")},
				{ResourceName: "velodb_warehouse.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"admin_password", "admin_password_version", "initial_cluster", "initial_cluster_id", "timeouts"}},
			}})
		})
	}
}

func TestAccNodeCapacityDataSource(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSONResponse(t, w, map[string]any{"success": true, "total": 3, "data": []any{
			map[string]any{"clusterId": "ebs", "warehouseId": "w", "name": "ebs", "status": "Running", "cacheType": 0, "nodeVcpu": 8, "nodeCount": 2, "nodeCache": 200, "capacityModel": "NODE_SPEC"},
			map[string]any{"clusterId": "local", "warehouseId": "w", "name": "local", "status": "Running", "cacheType": 1, "nodeVcpu": 4, "nodeCount": 2, "nodeCache": 237, "capacityModel": "NODE_SPEC"},
			map[string]any{"clusterId": "older", "warehouseId": "w", "name": "older", "status": "Running", "nodeCount": 1},
		}})
	}))
	defer ts.Close()
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts), Steps: []resource.TestStep{{
		Config: testProviderConfig(ts) + `data "velodb_clusters" "test" { warehouse_id="w" }`,
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.velodb_clusters.test", "clusters.0.cache_type", "0"),
			resource.TestCheckResourceAttr("data.velodb_clusters.test", "clusters.1.cache_type", "1"),
			resource.TestCheckResourceAttr("data.velodb_clusters.test", "clusters.1.node_cache", "237"),
			resource.TestCheckNoResourceAttr("data.velodb_clusters.test", "clusters.2.cache_type"),
			resource.TestCheckNoResourceAttr("data.velodb_clusters.test", "clusters.2.node_cache"),
		),
	}}})
}
