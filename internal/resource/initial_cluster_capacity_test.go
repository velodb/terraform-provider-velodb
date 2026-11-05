package resource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestInitialClusterCapacityUpdate(t *testing.T) {
	for _, scenario := range []string{"legacy CPU", "legacy cache", "node size", "node count", "node cache", "local CPU", "node migration", "both dimensions", "remote drift changes both", "legacy on NODE_SPEC", "mixed billing", "missing API fields", "missing ID", "wrong warehouse", "wrong type", "deleted", "resize rejected", "partial progress", "old Running", "poll failed", "later rename failed", "recovery unavailable", "noop", "promotion rejected"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			if scenario == "old Running" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
			}
			node := !strings.HasPrefix(scenario, "legacy")
			cpu, cache, count := 4, 100, 2
			if node {
				cpu, cache = 8, 200
			}
			local := scenario == "local CPU"
			if local {
				cache = 474
			}
			patches := 0
			failed := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				reject := func(code int) {
					w.WriteHeader(code)
					_ = json.NewEncoder(w).Encode(map[string]any{"code": "OperationConflict", "message": "test failure", "requestId": "resize-request"})
				}
				if req.URL.Path == "/v1/warehouses/w" {
					if req.Method == http.MethodPatch {
						failed = true
						reject(409)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"warehouseId": "w", "name": "analytics", "status": "Running"}})
					return
				}
				if req.URL.Path != "/v1/warehouses/w/clusters/c" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					reject(400)
					return
				}
				if req.Method == http.MethodPatch {
					patches++
					var body map[string]any
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body["nodeCount"] != nil && (body["nodeVcpu"] != nil || body["nodeCache"] != nil) {
						t.Errorf("combined dimensions: %v", body)
					}
					if node && (body["computeVcpu"] != nil || body["cacheGb"] != nil) {
						t.Errorf("mixed families: %v", body)
					}
					if local && body["nodeCache"] != nil {
						t.Errorf("local cache leaked: %v", body)
					}
					if scenario == "resize rejected" || scenario == "promotion rejected" {
						reject(409)
						return
					}
					if scenario != "old Running" {
						for key, dst := range map[string]*int{"nodeVcpu": &cpu, "nodeCache": &cache, "nodeCount": &count, "computeVcpu": &cpu, "cacheGb": &cache} {
							if v, ok := body[key].(float64); ok {
								*dst = int(v)
							}
						}
						if body["computeVcpu"] != nil {
							cache = 200
						}
						if local {
							cache = 777
						}
					}
					if scenario == "partial progress" {
						reject(409)
						return
					}
					_, _ = w.Write([]byte(`{"success":true}`))
					return
				}
				if scenario == "deleted" {
					reject(404)
					return
				}
				if scenario == "recovery unavailable" && failed || scenario == "poll failed" && patches > 0 {
					reject(503)
					return
				}
				warehouseID, kind := "w", "COMPUTE"
				if scenario == "wrong warehouse" {
					warehouseID = "other"
				}
				if scenario == "wrong type" {
					kind = "SQL"
				}
				data := map[string]any{"clusterId": "c", "warehouseId": warehouseID, "clusterType": kind, "status": "Running", "billingSummary": map[string]any{"totalCpu": cpu, "totalDiskSizeGb": cache, "nodeCount": count}}
				if node || scenario == "legacy on NODE_SPEC" {
					data["nodeVcpu"], data["nodeCache"], data["nodeCount"], data["capacityModel"], data["cacheStorageType"] = cpu, cache, count, "NODE_SPEC", 0
					if (scenario == "node migration" && patches == 0) || scenario == "promotion rejected" {
						delete(data, "capacityModel")
					}
					if local {
						data["cacheStorageType"] = 1
					}
					if scenario == "missing API fields" {
						delete(data, "nodeVcpu")
					}
					if scenario == "mixed billing" {
						data["onDemandNodeCount"], data["subscriptionNodeCount"] = 1, 1
					}
					if scenario == "remote drift changes both" {
						data["nodeVcpu"] = 16
					}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
			}))
			defer server.Close()
			r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "key", 0, time.Second)}
			prior := warehouseIdentityTestState(t)
			for name, value := range map[string]any{"initial_cluster_capacity_adopted": true, "id": "w", "initial_cluster_id": "c", "name": "analytics", "initial_cluster": initialClusterListForTest(types.Int64Value(8))} {
				if d := prior.SetAttribute(ctx, path.Root(name), value); d.HasError() {
					t.Fatal(d)
				}
			}
			set := func(s *tfsdk.State, name string, value types.Int64) {
				t.Helper()
				if d := s.SetAttribute(ctx, path.Root("initial_cluster").AtListIndex(0).AtName(name), value); d.HasError() {
					t.Fatal(d)
				}
			}
			if node {
				for key, value := range map[string]types.Int64{"compute_vcpu": types.Int64Null(), "cache_gb": types.Int64Null(), "node_vcpu": types.Int64Value(8), "node_count": types.Int64Value(2), "node_cache": types.Int64Value(int64(cache)), "cache_storage_type": types.Int64Value(0)} {
					set(&prior, key, value)
				}
				if local {
					set(&prior, "cache_storage_type", types.Int64Value(1))
				}
			}
			if scenario == "missing ID" {
				if d := prior.SetAttribute(ctx, path.Root("initial_cluster_id"), types.StringNull()); d.HasError() {
					t.Fatal(d)
				}
			}
			planned := prior
			key, value := "node_count", types.Int64Value(3)
			switch scenario {
			case "legacy CPU", "legacy on NODE_SPEC":
				set(&planned, "compute_vcpu", types.Int64Value(8))
				key, value = "cache_gb", types.Int64Value(200)
			case "legacy cache":
				key, value = "cache_gb", types.Int64Value(200)
			case "node size", "local CPU":
				key, value = "node_vcpu", types.Int64Value(16)
			case "node cache":
				key, value = "node_cache", types.Int64Value(300)
			case "both dimensions":
				set(&planned, "node_vcpu", types.Int64Value(16))
			case "node migration", "promotion rejected":
				value = types.Int64Value(2)
			case "noop":
				value = types.Int64Value(2)
			}
			set(&planned, key, value)
			if scenario == "later rename failed" || scenario == "recovery unavailable" {
				if d := planned.SetAttribute(ctx, path.Root("name"), "renamed"); d.HasError() {
					t.Fatal(d)
				}
			}
			configured := planned
			if local {
				set(&configured, "node_cache", types.Int64Null())
				set(&planned, "node_cache", types.Int64Unknown())
			}
			response := resource.UpdateResponse{State: prior}
			r.Update(ctx, resource.UpdateRequest{State: prior, Plan: tfsdk.Plan(planned), Config: tfsdk.Config(configured)}, &response)
			wantError := true
			switch scenario {
			case "legacy CPU", "legacy cache", "node size", "node count", "node cache", "local CPU", "node migration", "noop":
				wantError = false
			}
			if response.Diagnostics.HasError() != wantError {
				t.Fatalf("diagnostics=%v", response.Diagnostics)
			}
			wantPatches := 1
			switch scenario {
			case "both dimensions", "remote drift changes both", "legacy on NODE_SPEC", "mixed billing", "missing API fields", "missing ID", "wrong warehouse", "wrong type", "deleted", "noop":
				wantPatches = 0
			}
			if patches != wantPatches {
				t.Fatalf("patches=%d want %d; diagnostics=%v", patches, wantPatches, response.Diagnostics)
			}
			var got WarehouseResourceModel
			if d := response.State.Get(context.Background(), &got); d.HasError() {
				t.Fatal(d)
			}
			if got.ID.ValueString() != "w" || scenario != "missing ID" && scenario != "deleted" && got.InitialClusterID.ValueString() != "c" {
				t.Fatalf("identity lost: %+v", got)
			}
			if scenario == "deleted" && !got.InitialClusterID.IsNull() {
				t.Fatal("deleted cluster ID must be cleared")
			}
			var d diag.Diagnostics
			capacity := initialCapacity(context.Background(), got.InitialCluster, &d)
			wantCount := int64(count)
			if scenario == "poll failed" {
				wantCount = 2
			} // No API read confirmed the accepted operation.
			if node && capacity.NodeCount.ValueInt64() != wantCount {
				t.Fatalf("confirmed count lost: %v want %d; diagnostics=%v", capacity.NodeCount, wantCount, response.Diagnostics)
			}
			if !node && capacity.ComputeVcpu.ValueInt64() != int64(cpu) {
				t.Fatalf("confirmed CPU lost: %+v", capacity)
			}
			if scenario == "promotion rejected" {
				retry := resource.ModifyPlanResponse{Plan: tfsdk.Plan(planned)}
				r.ModifyPlan(context.Background(), resource.ModifyPlanRequest{State: response.State, Plan: tfsdk.Plan(planned), Config: tfsdk.Config(configured)}, &retry)
				var pending WarehouseResourceModel
				if d := retry.Plan.Get(context.Background(), &pending); d.HasError() {
					t.Fatal(d)
				}
				var pd diag.Diagnostics
				block := initialCapacity(context.Background(), pending.InitialCluster, &pd)
				if retry.Diagnostics.HasError() || block == nil || !block.CapacityModel.IsUnknown() {
					t.Fatalf("promotion must remain pending: %v %+v", retry.Diagnostics, block)
				}
			}
			if scenario == "old Running" && !strings.Contains(response.Diagnostics.Errors()[0].Summary(), "incomplete") {
				t.Fatalf("stale Running reported success: %v", response.Diagnostics)
			}
		})
	}
}

func TestInitialClusterCapacityPlan(t *testing.T) {
	for _, scenario := range []string{"both dimensions", "unknown count", "import adoption", "remove block", "ratio", "cache storage type", "default cache storage type"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			state := warehouseIdentityTestState(t)
			for name, value := range map[string]any{"initial_cluster_capacity_adopted": true, "id": "w", "initial_cluster_id": "c", "initial_cluster": initialClusterListForTest(types.Int64Null())} {
				if d := state.SetAttribute(ctx, path.Root(name), value); d.HasError() {
					t.Fatal(d)
				}
			}
			for name, value := range map[string]int64{"node_vcpu": 8, "node_count": 2, "node_cache": 200, "cache_storage_type": 0} {
				if d := state.SetAttribute(ctx, path.Root("initial_cluster").AtListIndex(0).AtName(name), types.Int64Value(value)); d.HasError() {
					t.Fatal(d)
				}
			}
			planned := state
			set := func(name string, value types.Int64) {
				if d := planned.SetAttribute(ctx, path.Root("initial_cluster").AtListIndex(0).AtName(name), value); d.HasError() {
					t.Fatal(d)
				}
			}
			switch scenario {
			case "both dimensions":
				set("node_count", types.Int64Value(3))
				set("node_vcpu", types.Int64Value(16))
			case "unknown count":
				set("node_count", types.Int64Unknown())
			case "ratio":
				set("ratio", types.Int64Value(4))
			case "cache storage type":
				set("cache_storage_type", types.Int64Value(1))
			case "default cache storage type":
				if d := state.SetAttribute(ctx, path.Root("initial_cluster").AtListIndex(0).AtName("cache_storage_type"), types.Int64Null()); d.HasError() {
					t.Fatal(d)
				}
			}
			var original WarehouseResourceModel
			if d := state.Get(ctx, &original); d.HasError() {
				t.Fatal(d)
			}
			if scenario == "import adoption" {
				original.InitialCluster = types.ListNull(original.InitialCluster.ElementType(ctx))
				if d := state.Set(ctx, &original); d.HasError() {
					t.Fatal(d)
				}
			}
			if scenario == "remove block" {
				if d := planned.SetAttribute(ctx, path.Root("initial_cluster"), types.ListNull(original.InitialCluster.ElementType(ctx))); d.HasError() {
					t.Fatal(d)
				}
			}
			result := resource.ModifyPlanResponse{Plan: tfsdk.Plan(planned)}
			(&WarehouseResource{}).ModifyPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: tfsdk.Plan(planned), Config: tfsdk.Config(planned)}, &result)
			wantError := scenario != "unknown count" && scenario != "remove block" && scenario != "default cache storage type"
			if result.Diagnostics.HasError() != wantError || len(result.RequiresReplace) != 0 {
				t.Fatalf("diagnostics=%v replacement=%v", result.Diagnostics, result.RequiresReplace)
			}
		})
	}
}

func TestInitialClusterCapacityRefresh(t *testing.T) {
	for _, scenario := range []string{"legacy stale state", "node drift", "imported", "missing ID", "missing capacity"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			clusterReads := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data := map[string]any{"warehouseId": "w", "name": "analytics", "status": "Running"}
				if r.URL.Path != "/v1/warehouses/w" {
					clusterReads++
					if r.URL.Path != "/v1/warehouses/w/clusters/c" || r.Method != http.MethodGet {
						t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					}
					data["clusterId"], data["clusterType"] = "c", "COMPUTE"
					if scenario != "missing capacity" {
						data["billingSummary"] = map[string]int{"totalCpu": 4, "totalDiskSizeGb": 100, "nodeCount": 2}
					}
					if scenario == "node drift" {
						data["nodeVcpu"], data["nodeCount"], data["nodeCache"], data["cacheStorageType"], data["capacityModel"] = 8, 2, 200, 0, "NODE_SPEC"
					}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
			}))
			defer server.Close()
			prior := warehouseIdentityTestState(t)
			for key, value := range map[string]any{"initial_cluster_capacity_adopted": true, "id": "w", "initial_cluster_id": "c", "initial_cluster": initialClusterListForTest(types.Int64Null())} {
				if d := prior.SetAttribute(ctx, path.Root(key), value); d.HasError() {
					t.Fatal(d)
				}
			}
			if scenario == "imported" {
				var model WarehouseResourceModel
				if d := prior.Get(ctx, &model); d.HasError() {
					t.Fatal(d)
				}
				model.InitialCluster = types.ListNull(model.InitialCluster.ElementType(ctx))
				model.InitialClusterID = types.StringNull()
				if d := prior.Set(ctx, &model); d.HasError() {
					t.Fatal(d)
				}
			}
			if scenario == "missing ID" {
				if d := prior.SetAttribute(ctx, path.Root("initial_cluster_id"), types.StringNull()); d.HasError() {
					t.Fatal(d)
				}
			}
			if scenario == "legacy stale state" {
				if d := prior.SetAttribute(ctx, path.Root("initial_cluster").AtListIndex(0).AtName("compute_vcpu"), types.Int64Value(8)); d.HasError() {
					t.Fatal(d)
				}
			}
			if scenario == "node drift" {
				for key, value := range map[string]types.Int64{"compute_vcpu": types.Int64Null(), "cache_gb": types.Int64Null(), "node_vcpu": types.Int64Value(16), "node_count": types.Int64Value(3), "node_cache": types.Int64Value(400)} {
					if d := prior.SetAttribute(ctx, path.Root("initial_cluster").AtListIndex(0).AtName(key), value); d.HasError() {
						t.Fatal(d)
					}
				}
			}
			r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "key", 0, time.Second)}
			response := resource.ReadResponse{State: prior}
			r.Read(ctx, resource.ReadRequest{State: prior}, &response)
			if response.Diagnostics.HasError() != (scenario == "missing capacity") {
				t.Fatal(response.Diagnostics)
			}
			wantReads := 1
			if scenario == "imported" || scenario == "missing ID" {
				wantReads = 0
			}
			if clusterReads != wantReads {
				t.Fatalf("cluster reads=%d want %d", clusterReads, wantReads)
			}
			var result WarehouseResourceModel
			if d := response.State.Get(ctx, &result); d.HasError() {
				t.Fatal(d)
			}
			var d diag.Diagnostics
			block := initialCapacity(ctx, result.InitialCluster, &d)
			if scenario == "legacy stale state" && block.ComputeVcpu.ValueInt64() != 4 {
				t.Fatalf("stale state not corrected: %v", block.ComputeVcpu)
			}
			if scenario == "node drift" && (block.NodeVcpu.ValueInt64() != 8 || block.NodeCount.ValueInt64() != 2 || block.NodeCache.ValueInt64() != 200) {
				t.Fatalf("node drift not read: %+v", block)
			}
		})
	}
}
