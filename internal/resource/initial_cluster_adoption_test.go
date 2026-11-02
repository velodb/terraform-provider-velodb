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

func TestInitialCapacityAdoptionProtectsExistingState(t *testing.T) {
	ctx := context.Background()
	cpu, cache, writes := 8, 200, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			writes++
			t.Errorf("adoption must not mutate infrastructure: %s", req.Method)
		}
		if strings.Contains(req.URL.Path, "/clusters/") {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"clusterId": "c", "warehouseId": "w", "clusterType": "COMPUTE", "status": "Running", "billingSummary": map[string]int{"totalCpu": cpu, "totalDiskSizeGb": cache}}})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"warehouseId": "w", "name": "analytics", "status": "Running"}})
		}
	}))
	defer server.Close()
	r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "key", 0, time.Second)}
	state := warehouseIdentityTestState(t)
	for key, value := range map[string]any{"id": "w", "name": "analytics", "initial_cluster_id": "c", "initial_cluster": initialClusterListForTest(types.Int64Null())} {
		if d := state.SetAttribute(ctx, path.Root(key), value); d.HasError() {
			t.Fatal(d)
		}
	}
	original := tfsdk.Config(state)
	for attempt := 0; attempt < 2; attempt++ {
		read := resource.ReadResponse{State: state}
		r.Read(ctx, resource.ReadRequest{State: state}, &read)
		if read.Diagnostics.HasError() {
			t.Fatal(read.Diagnostics)
		}
		state = read.State
		plan := tfsdk.Plan(state)
		if d := plan.SetAttribute(ctx, path.Root("initial_cluster"), initialClusterListForTest(types.Int64Null())); d.HasError() {
			t.Fatal(d)
		}
		result := resource.ModifyPlanResponse{Plan: plan}
		r.ModifyPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: plan, Config: original}, &result)
		if !result.Diagnostics.HasError() || !strings.Contains(result.Diagnostics.Errors()[0].Summary(), "adoption required") {
			t.Fatalf("stale settings must stop every plan until aligned: %v", result.Diagnostics)
		}
	}
	// Align configuration to observed values. A metadata-only apply adopts management.
	config := tfsdk.Config(state)
	plan := tfsdk.Plan(state)
	result := resource.ModifyPlanResponse{Plan: plan}
	r.ModifyPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: plan, Config: config}, &result)
	if result.Diagnostics.HasError() {
		t.Fatal(result.Diagnostics)
	}
	var adopted types.Bool
	if d := result.Plan.GetAttribute(ctx, path.Root("initial_cluster_capacity_adopted"), &adopted); d.HasError() || !adopted.ValueBool() {
		t.Fatalf("missing planned adoption: %v", d)
	}
	// Remote changes after planning cannot sneak through adoption at apply.
	cache = 300
	update := resource.UpdateResponse{State: state}
	r.Update(ctx, resource.UpdateRequest{State: state, Plan: result.Plan, Config: config}, &update)
	if !update.Diagnostics.HasError() {
		t.Fatal("saved plan bypassed the adoption check")
	}
	cache = 200
	update = resource.UpdateResponse{State: state}
	r.Update(ctx, resource.UpdateRequest{State: state, Plan: result.Plan, Config: config}, &update)
	if update.Diagnostics.HasError() {
		t.Fatal(update.Diagnostics)
	}
	if d := update.State.GetAttribute(ctx, path.Root("initial_cluster_capacity_adopted"), &adopted); d.HasError() || !adopted.ValueBool() || writes != 0 {
		t.Fatalf("adoption must persist without writes: %v writes=%d", d, writes)
	}
}

func TestInitialCapacityAdoptionFields(t *testing.T) {
	ctx := context.Background()
	for _, field := range []string{"compute_vcpu", "cache_gb", "node_vcpu", "node_count", "node_cache"} {
		t.Run(field, func(t *testing.T) {
			state := warehouseIdentityTestState(t)
			if d := state.SetAttribute(ctx, path.Root("initial_cluster"), initialClusterListForTest(types.Int64Null())); d.HasError() {
				t.Fatal(d)
			}
			if d := state.SetAttribute(ctx, path.Root("initial_cluster").AtListIndex(0).AtName(field), types.Int64Value(8)); d.HasError() {
				t.Fatal(d)
			}
			var actual types.List
			if d := state.GetAttribute(ctx, path.Root("initial_cluster"), &actual); d.HasError() {
				t.Fatal(d)
			}
			for _, value := range []types.Int64{types.Int64Value(4), types.Int64Unknown()} {
				if d := state.SetAttribute(ctx, path.Root("initial_cluster").AtListIndex(0).AtName(field), value); d.HasError() {
					t.Fatal(d)
				}
				var config types.List
				if d := state.GetAttribute(ctx, path.Root("initial_cluster"), &config); d.HasError() {
					t.Fatal(d)
				}
				var d diag.Diagnostics
				if initialCapacityMatches(ctx, config, actual, &d) || d.HasError() == value.IsUnknown() {
					t.Fatalf("invalid adoption for %s=%s: %v", field, value, d)
				}
				if !value.IsUnknown() {
					detail := d.Errors()[0].Detail()
					if !strings.Contains(detail, "configuration specifies 4") || !strings.Contains(detail, "set "+field+" = 8 and apply once. You can resize afterward.") || strings.Contains(detail, "Remove initial_cluster") {
						t.Fatalf("mismatch must give the exact correction without ownership advice: %s", detail)
					}
					if (field == "cache_gb" || field == "node_cache") && !strings.Contains(detail, "8 GB; configuration specifies 4 GB") {
						t.Fatalf("cache mismatch must include GB units: %s", detail)
					}
				}
			}
		})
	}
}

func TestWarehouseCreateClusterObservationFailure(t *testing.T) {
	for _, scenario := range []string{"forbidden", "unavailable", "cluster failed", "warehouse failed"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch {
				case req.Method == http.MethodPost:
					_, _ = w.Write([]byte(`{"success":true,"data":{"warehouseId":"w"}}`))
				case strings.HasSuffix(req.URL.Path, "/clusters"):
					_, _ = w.Write([]byte(`{"success":true,"total":1,"data":[{"clusterId":"c","warehouseId":"w","clusterType":"COMPUTE"}]}`))
				case strings.Contains(req.URL.Path, "/clusters/"):
					if scenario == "cluster failed" {
						_, _ = w.Write([]byte(`{"success":true,"data":{"clusterId":"c","warehouseId":"w","clusterType":"COMPUTE","status":"CreateFailed"}}`))
						return
					}
					status := 403
					if scenario == "unavailable" {
						status = 503
					}
					w.WriteHeader(status)
					_, _ = w.Write([]byte(`{"code":"Forbidden","message":"capacity read denied","requestId":"read-request"}`))
				default:
					status := "Running"
					if scenario == "warehouse failed" {
						status = "CreateFailed"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"warehouseId": "w", "status": status}})
				}
			}))
			defer server.Close()
			ctx := context.Background()
			state := warehouseIdentityTestState(t)
			plan := tfsdk.Plan(state)
			for key, value := range map[string]any{"name": "analytics", "deployment_mode": "SaaS", "cloud_provider": "aws", "region": "us-east-1", "admin_password": "TestPass@123", "initial_cluster": initialClusterListForTest(types.Int64Null())} {
				if d := plan.SetAttribute(ctx, path.Root(key), value); d.HasError() {
					t.Fatal(d)
				}
			}
			r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "key", 0, time.Second)}
			response := resource.CreateResponse{State: state}
			r.Create(ctx, resource.CreateRequest{Plan: plan}, &response)
			var got WarehouseResourceModel
			if d := response.State.Get(ctx, &got); d.HasError() {
				t.Fatal(d)
			}
			failed := strings.Contains(scenario, "failed")
			if response.Diagnostics.HasError() != failed || got.ID.ValueString() != "w" || got.InitialClusterID.ValueString() != "c" || got.InitialCapacityAdopted.ValueBool() {
				t.Fatalf("incorrect create outcome: %v state=%+v", response.Diagnostics, got)
			}
			if !failed && (len(response.Diagnostics.Warnings()) == 0 || !strings.Contains(response.Diagnostics.Warnings()[0].Detail(), "read-request")) {
				t.Fatalf("missing actionable warning: %v", response.Diagnostics)
			}
		})
	}
}

func TestInitialCapacityAdoptionDefersUnknownPlan(t *testing.T) {
	ctx := context.Background()
	state := warehouseIdentityTestState(t)
	for key, value := range map[string]any{"id": "w", "initial_cluster_id": "c", "initial_cluster": initialClusterListForTest(types.Int64Null())} {
		if d := state.SetAttribute(ctx, path.Root(key), value); d.HasError() {
			t.Fatal(d)
		}
	}
	plan := tfsdk.Plan(state)
	if d := plan.SetAttribute(ctx, path.Root("initial_cluster").AtListIndex(0).AtName("compute_vcpu"), types.Int64Unknown()); d.HasError() {
		t.Fatal(d)
	}
	result := resource.ModifyPlanResponse{Plan: plan}
	(&WarehouseResource{}).ModifyPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: plan, Config: tfsdk.Config(plan)}, &result)
	var adopted types.Bool
	if d := result.Plan.GetAttribute(ctx, path.Root("initial_cluster_capacity_adopted"), &adopted); d.HasError() || result.Diagnostics.HasError() || !adopted.IsUnknown() {
		t.Fatalf("adoption must remain unknown until capacity inputs resolve: adopted=%v diagnostics=%v %v", adopted, result.Diagnostics, d)
	}
}
