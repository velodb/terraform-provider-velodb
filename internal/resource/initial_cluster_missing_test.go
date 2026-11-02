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

func TestInitialClusterReadErrorClassification(t *testing.T) {
	for _, status := range []int{404, 403, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			reads := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reads++
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(map[string]any{"code": "ClusterNotFound", "message": "initial cluster unavailable"})
			}))
			defer server.Close()
			ctx := context.Background()
			state := WarehouseResourceModel{ID: types.StringValue("w"), InitialClusterID: types.StringValue("c"), InitialCluster: initialClusterListForTest(types.Int64Null())}
			before := state.InitialCluster
			r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "key", 0, time.Second)}
			var d diag.Diagnostics
			r.readInitialCapacity(ctx, &state, &d)
			if d.HasError() != (status != 404) || state.InitialClusterID.IsNull() != (status == 404) || state.ID.ValueString() != "w" || !state.InitialCluster.Equal(before) {
				t.Fatalf("warehouse/last-known capacity must survive; only 404 clears cluster ID: %+v %v", state, d)
			}
			if status == 404 {
				if len(d.Warnings()) != 1 {
					t.Fatalf("missing warning: %v", d)
				}
				r.readInitialCapacity(ctx, &state, &d)
				if reads != 1 {
					t.Fatalf("missing target must not be rediscovered: reads=%d", reads)
				}
			}
		})
	}
}

func TestDeletedInitialClusterAllowsWarehouseLifecycle(t *testing.T) {
	ctx := context.Background()
	childReads, capacityWrites, deletes := 0, 0, 0
	name := "analytics"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/warehouses/w/clusters/c" {
			childReads++
			if r.Method != http.MethodGet {
				capacityWrites++
			}
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"code":"ClusterNotFound","message":"deleted"}`))
			return
		}
		if r.URL.Path == "/v1/warehouses" && deletes > 0 {
			_, _ = w.Write([]byte(`{"success":true,"data":[],"total":0}`))
			return
		}
		if r.URL.Path != "/v1/warehouses/w" {
			t.Errorf("unexpected target %s", r.URL.Path)
			w.WriteHeader(400)
			return
		}
		if r.Method == http.MethodDelete {
			deletes++
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		}
		if deletes > 0 {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"code":"WarehouseNotFound"}`))
			return
		}
		if r.Method == http.MethodPatch {
			var update struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				t.Error(err)
			}
			name = update.Name
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"warehouseId": "w", "name": name, "status": "Running"}})
	}))
	defer server.Close()
	r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "key", 0, time.Second)}
	state := warehouseIdentityTestState(t)
	for key, value := range map[string]any{"id": "w", "initial_cluster_id": "c", "name": "analytics", "initial_cluster": initialClusterListForTest(types.Int64Null())} {
		if d := state.SetAttribute(ctx, path.Root(key), value); d.HasError() {
			t.Fatal(d)
		}
	}
	// Include a pending legacy-to-node promotion: a missing target must not perpetually plan it.
	for key, value := range map[string]types.Int64{"compute_vcpu": types.Int64Null(), "cache_gb": types.Int64Null(), "node_vcpu": types.Int64Value(8), "node_count": types.Int64Value(2), "node_cache": types.Int64Value(200)} {
		if d := state.SetAttribute(ctx, path.Root("initial_cluster").AtListIndex(0).AtName(key), value); d.HasError() {
			t.Fatal(d)
		}
	}
	read := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	state = read.State
	for _, change := range []string{"unchanged", "rename", "resize", "remove"} {
		planned := tfsdk.Plan(state)
		switch change {
		case "rename":
			if d := planned.SetAttribute(ctx, path.Root("name"), "renamed"); d.HasError() {
				t.Fatal(d)
			}
		case "resize":
			if d := planned.SetAttribute(ctx, path.Root("initial_cluster").AtListIndex(0).AtName("node_count"), types.Int64Value(3)); d.HasError() {
				t.Fatal(d)
			}
		case "remove":
			var model WarehouseResourceModel
			if d := state.Get(ctx, &model); d.HasError() {
				t.Fatal(d)
			}
			if d := planned.SetAttribute(ctx, path.Root("initial_cluster"), types.ListNull(model.InitialCluster.ElementType(ctx))); d.HasError() {
				t.Fatal(d)
			}
		}
		config := tfsdk.Config(planned)
		modified := resource.ModifyPlanResponse{Plan: planned}
		r.ModifyPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: planned, Config: config}, &modified)
		if modified.Diagnostics.HasError() != (change == "resize") || len(modified.RequiresReplace) != 0 {
			t.Fatalf("%s plan: %v", change, modified.Diagnostics)
		}
		var model WarehouseResourceModel
		if d := modified.Plan.Get(ctx, &model); d.HasError() {
			t.Fatal(d)
		}
		if change != "remove" && change != "resize" {
			var d diag.Diagnostics
			block := initialCapacity(ctx, model.InitialCluster, &d)
			if block.CapacityModel.IsUnknown() {
				t.Fatal("missing target scheduled a model promotion")
			}
		}
		update := resource.UpdateResponse{State: state}
		r.Update(ctx, resource.UpdateRequest{State: state, Plan: modified.Plan, Config: config}, &update)
		if update.Diagnostics.HasError() != (change == "resize") {
			t.Fatalf("%s update: %v", change, update.Diagnostics)
		}
		if change != "resize" {
			state = update.State
		}
	}
	deletion := resource.DeleteResponse{State: state}
	r.Delete(ctx, resource.DeleteRequest{State: state}, &deletion)
	if deletion.Diagnostics.HasError() || childReads != 1 || capacityWrites != 0 || deletes != 1 {
		t.Fatalf("unexpected operations childReads=%d capacityWrites=%d deletes=%d diagnostics=%v", childReads, capacityWrites, deletes, deletion.Diagnostics)
	}
}
