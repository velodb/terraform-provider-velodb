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
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func nodePlan() ClusterResourceModel {
	return ClusterResourceModel{NodeVcpu: types.Int64Value(8), NodeCount: types.Int64Value(3), NodeCache: types.Int64Value(200)}
}

func TestValidateCapacityFamilies(t *testing.T) {
	for _, tc := range []struct {
		name                                  string
		cpu, cache, nodeCPU, count, nodeCache types.Int64
		invalid                               bool
	}{
		{name: "legacy", cpu: types.Int64Value(16), cache: types.Int64Value(200)},
		{name: "node", nodeCPU: types.Int64Value(8), count: types.Int64Value(3), nodeCache: types.Int64Value(200)},
		{name: "unknown node", nodeCPU: types.Int64Unknown(), count: types.Int64Unknown(), nodeCache: types.Int64Unknown()},
		{name: "missing capacity", invalid: true},
		{name: "missing node cache", nodeCPU: types.Int64Value(8), count: types.Int64Value(3), invalid: true},
		{name: "explicit legacy zero", cpu: types.Int64Value(0), nodeCPU: types.Int64Value(8), count: types.Int64Value(3), nodeCache: types.Int64Value(200), invalid: true},
		{name: "zero count", nodeCPU: types.Int64Value(8), count: types.Int64Value(0), nodeCache: types.Int64Value(200), invalid: true},
		{name: "CPU overflow", nodeCPU: types.Int64Value(2147483648), count: types.Int64Value(1), nodeCache: types.Int64Value(200), invalid: true},
		{name: "aggregate overflow", nodeCPU: types.Int64Value(2147483647), count: types.Int64Value(2), nodeCache: types.Int64Value(200), invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var d diag.Diagnostics
			validateCapacity(&d, tc.cpu, tc.cache, tc.nodeCPU, tc.count, tc.nodeCache, types.Int64Null())
			if d.HasError() != tc.invalid {
				t.Fatalf("diagnostics: %v", d)
			}
		})
	}
}

func TestNodeCapacityGuardsAndTimeout(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		missing, mixed, manual, timeout bool
		wantMutations                   int
	}{
		{name: "missing API fields", missing: true},
		{name: "mixed billing", mixed: true},
		{name: "cache-only manual intent", manual: true},
		{name: "stale running status", timeout: true, wantMutations: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutations := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPatch {
					mutations++
					_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
					return
				}
				data := map[string]any{"status": "Running", "capacityModel": "NODE_SPEC", "nodeCount": 3, "nodeVcpu": 8, "nodeCache": 200}
				if tc.missing {
					delete(data, "nodeCache")
				}
				if tc.mixed {
					data["onDemandNodeCount"] = 1
					data["subscriptionNodeCount"] = 2
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
			}))
			defer server.Close()
			r := &ClusterResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "test-key", 0, time.Second)}
			prior := nodePlan()
			next := nodePlan()
			next.NodeCache = types.Int64Value(300)
			if tc.manual {
				next.ScalingMode = types.StringValue("MANUAL")
			}
			var d diag.Diagnostics
			if r.updateNodeCapacity(context.Background(), "w", "c", next, &next, prior, 20*time.Millisecond, &d) || !d.HasError() {
				t.Fatalf("expected rejection/incomplete operation: %v", d)
			}
			if mutations != tc.wantMutations {
				t.Fatalf("mutations=%d, want %d", mutations, tc.wantMutations)
			}
		})
	}
}

func TestNodeCapacityReadClearsMissingValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"clusterId": "c", "nodeCount": 1, "status": "Running"}})
	}))
	defer server.Close()
	r := &ClusterResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "test-key", 0, time.Second)}
	state := nodePlan()
	state.CapacityModel = types.StringValue("NODE_SPEC")
	var d diag.Diagnostics
	r.readClusterIntoState(context.Background(), "w", "c", &state, &d)
	if d.HasError() || !state.NodeVcpu.IsNull() || !state.NodeCache.IsNull() || !state.CapacityModel.IsNull() || state.NodeCount.ValueInt64() != 1 {
		t.Fatalf("missing fields must remain unknown/unrecorded: %+v %v", state, d)
	}
}

func TestNodeUpdateFailureRefreshesAppliedCapacity(t *testing.T) {
	ctx := context.Background()
	applied := 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPatch {
			applied = 300
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "ResizeFailed", "message": "partial progress"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"clusterId": "c", "warehouseId": "w", "name": "analytics", "clusterType": "COMPUTE", "status": "Running", "cacheType": 0, "nodeVcpu": 8, "nodeCount": 3, "nodeCache": applied, "capacityModel": "NODE_SPEC"}})
	}))
	defer server.Close()
	r := &ClusterResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "test-key", 0, time.Second)}
	var schemaResponse resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
	ty := schemaResponse.Schema.Type().TerraformType(ctx).(tftypes.Object)
	values := map[string]tftypes.Value{}
	for key, attrType := range ty.AttributeTypes {
		values[key] = tftypes.NewValue(attrType, nil)
	}
	priorState := tfsdk.State{Schema: schemaResponse.Schema, Raw: tftypes.NewValue(ty, values)}
	var prior ClusterResourceModel
	if d := priorState.Get(ctx, &prior); d.HasError() {
		t.Fatal(d)
	}
	prior.ID = types.StringValue("c")
	prior.WarehouseID = types.StringValue("w")
	prior.Name = types.StringValue("analytics")
	prior.ClusterType = types.StringValue("COMPUTE")
	prior.NodeVcpu = types.Int64Value(8)
	prior.NodeCount = types.Int64Value(3)
	prior.NodeCache = types.Int64Value(200)
	prior.CacheType = types.Int64Value(0)
	prior.CapacityModel = types.StringValue("NODE_SPEC")
	if d := priorState.Set(ctx, &prior); d.HasError() {
		t.Fatal(d)
	}
	next := prior
	next.NodeCache = types.Int64Value(300)
	planState := priorState
	if d := planState.Set(ctx, &next); d.HasError() {
		t.Fatal(d)
	}
	resp := resource.UpdateResponse{State: priorState}
	r.Update(ctx, resource.UpdateRequest{State: priorState, Plan: tfsdk.Plan(planState), Config: tfsdk.Config{Schema: schemaResponse.Schema, Raw: planState.Raw}}, &resp)
	var got ClusterResourceModel
	if d := resp.State.Get(ctx, &got); d.HasError() {
		t.Fatal(d)
	}
	if !resp.Diagnostics.HasError() || got.ID.ValueString() != "c" || got.NodeCache.ValueInt64() != 300 {
		t.Fatalf("partial progress lost: %+v %v", got, resp.Diagnostics)
	}
}

func TestNodeUpdateStateTransitionOrdering(t *testing.T) {
	for _, action := range []string{"resume", "pause"} {
		for _, scenario := range []string{"node_count", "node_vcpu", "node_cache", "action rejected", "action timeout", "resize rejected"} {
			t.Run(action+"/"+scenario, func(t *testing.T) {
				ctx := context.Background()
				if scenario == "action timeout" {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
					defer cancel()
				}
				status := "Suspended"
				if action == "pause" {
					status = "Running"
				}
				cpu, count, cache := 8, 3, 200
				actions, resizes := 0, 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/"+action):
						actions++
						if action == "pause" && resizes != 1 {
							t.Error("pause was sent before resizing")
						}
						if scenario == "action rejected" {
							w.WriteHeader(http.StatusConflict)
							_, _ = w.Write([]byte(`{"code":"OperationConflict","message":"action rejected"}`))
							return
						}
						status = "Transitioning"
						_, _ = w.Write([]byte(`{"success":true}`))
					case req.Method == http.MethodPatch:
						resizes++
						if status != "Running" || scenario == "resize rejected" {
							w.WriteHeader(http.StatusConflict)
							_, _ = w.Write([]byte(`{"code":"OperationConflict","message":"cluster is not running"}`))
							return
						}
						var update client.UpdateClusterRequest
						if err := json.NewDecoder(req.Body).Decode(&update); err != nil {
							t.Error(err)
						}
						if update.NodeCount != nil {
							count = *update.NodeCount
						}
						if update.NodeVcpu != nil {
							cpu = *update.NodeVcpu
						}
						if update.NodeCache != nil {
							cache = *update.NodeCache
						}
						_, _ = w.Write([]byte(`{"success":true}`))
					default:
						if status == "Transitioning" && scenario != "action timeout" {
							status = "Running"
							if action == "pause" {
								status = "Suspended"
							}
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{
							"clusterId": "c", "warehouseId": "w", "name": "analytics", "clusterType": "COMPUTE",
							"status": status, "cacheType": 0, "nodeVcpu": cpu, "nodeCount": count,
							"nodeCache": cache, "capacityModel": "NODE_SPEC",
						}})
					}
				}))
				defer server.Close()
				r := &ClusterResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "test-key", 0, time.Second)}
				var schemaResponse resource.SchemaResponse
				r.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
				ty := schemaResponse.Schema.Type().TerraformType(ctx).(tftypes.Object)
				values := map[string]tftypes.Value{}
				for key, attrType := range ty.AttributeTypes {
					values[key] = tftypes.NewValue(attrType, nil)
				}
				priorState := tfsdk.State{Schema: schemaResponse.Schema, Raw: tftypes.NewValue(ty, values)}
				var prior ClusterResourceModel
				if d := priorState.Get(ctx, &prior); d.HasError() {
					t.Fatal(d)
				}
				prior.ID = types.StringValue("c")
				prior.WarehouseID = types.StringValue("w")
				prior.Name = types.StringValue("analytics")
				prior.ClusterType = types.StringValue("COMPUTE")
				prior.NodeVcpu = types.Int64Value(8)
				prior.NodeCount = types.Int64Value(3)
				prior.NodeCache = types.Int64Value(200)
				prior.CacheType = types.Int64Value(0)
				prior.CapacityModel = types.StringValue("NODE_SPEC")
				prior.DesiredState = types.StringValue("paused")
				if action == "pause" {
					prior.DesiredState = types.StringValue("running")
				}
				if d := priorState.Set(ctx, &prior); d.HasError() {
					t.Fatal(d)
				}
				next := prior
				next.DesiredState = types.StringValue("running")
				if action == "pause" {
					next.DesiredState = types.StringValue("paused")
				}
				switch scenario {
				case "node_count":
					next.NodeCount = types.Int64Value(4)
				case "node_vcpu":
					next.NodeVcpu = types.Int64Value(16)
				default:
					next.NodeCache = types.Int64Value(300)
				}
				planState := priorState
				if d := planState.Set(ctx, &next); d.HasError() {
					t.Fatal(d)
				}
				resp := resource.UpdateResponse{State: priorState}
				r.Update(ctx, resource.UpdateRequest{State: priorState, Plan: tfsdk.Plan(planState), Config: tfsdk.Config{Schema: schemaResponse.Schema, Raw: planState.Raw}}, &resp)
				failedAction := strings.HasPrefix(scenario, "action ")
				wantActions, wantResizes := 1, 1
				if action == "resume" && failedAction {
					wantResizes = 0
				}
				if action == "pause" && scenario == "resize rejected" {
					wantActions = 0
				}
				wantError := failedAction || scenario == "resize rejected"
				if actions != wantActions || resizes != wantResizes || resp.Diagnostics.HasError() != wantError {
					t.Fatalf("actions=%d resizes=%d diagnostics=%v", actions, resizes, resp.Diagnostics)
				}
				if !wantError || action == "pause" && failedAction {
					var got ClusterResourceModel
					if d := resp.State.Get(context.Background(), &got); d.HasError() {
						t.Fatal(d)
					}
					if got.ID.ValueString() != "c" || !got.NodeCount.Equal(next.NodeCount) || !got.NodeVcpu.Equal(next.NodeVcpu) || !got.NodeCache.Equal(next.NodeCache) {
						t.Fatalf("applied capacity lost: %+v", got)
					}
					if !wantError && !got.DesiredState.Equal(next.DesiredState) {
						t.Fatalf("desired state mismatch: %+v", got)
					}
				}
			})
		}
	}
}
