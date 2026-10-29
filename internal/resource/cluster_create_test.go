package resource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestClusterCreateRetainsIdentityAndWaits(t *testing.T) {
	for _, scenario := range []string{"pause rejected", "final read failed", "creation failed before pause", "creation failed running", "pause polling failed", "create timeout", "pause timeout", "running succeeds", "paused succeeds"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			if strings.HasSuffix(scenario, "timeout") {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
			}
			gets, pauses := 0, 0
			status := "Running"
			if scenario == "create timeout" {
				status = "Creating"
			}
			if strings.HasPrefix(scenario, "creation failed") {
				status = "CreateFailed"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fail := func() {
					w.WriteHeader(409)
					_, _ = w.Write([]byte(`{"code":"OperationConflict","message":"operation rejected"}`))
				}
				switch {
				case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/pause"):
					pauses++
					if gets == 0 {
						t.Error("pause sent before checking readiness")
					}
					if scenario == "pause rejected" || status != "Running" {
						fail()
						return
					}
					status = "Suspending"
					if scenario == "paused succeeds" {
						status = "Suspended"
					}
					_, _ = w.Write([]byte(`{"success":true}`))
				case req.Method == http.MethodPost:
					_, _ = w.Write([]byte(`{"success":true,"data":{"clusterId":"c-created"}}`))
				default:
					gets++
					if scenario == "final read failed" && gets >= 2 || scenario == "pause polling failed" && gets == 2 {
						fail()
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{
						"clusterId": "c-created", "warehouseId": "w", "name": "analytics", "clusterType": "COMPUTE", "status": status,
						"nodeVcpu": 8, "nodeCount": 2, "nodeCache": 200, "cacheType": 0, "capacityModel": "NODE_SPEC",
					}})
				}
			}))
			defer server.Close()
			r := &ClusterResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "test", 0, time.Second)}
			var sch resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &sch)
			ty := sch.Schema.Type().TerraformType(ctx).(tftypes.Object)
			attrs := map[string]tftypes.Value{}
			for key, attr := range ty.AttributeTypes {
				attrs[key] = tftypes.NewValue(attr, nil)
			}
			empty := tfsdk.State{Schema: sch.Schema, Raw: tftypes.NewValue(ty, attrs)}
			planValues := make(map[string]tftypes.Value, len(attrs))
			for key, value := range attrs {
				planValues[key] = value
			}
			for _, key := range []string{"id", "status", "zone", "compute_vcpu", "cache_gb", "capacity_model", "total_cpu", "total_disk_gb", "connection_info"} {
				planValues[key] = tftypes.NewValue(ty.AttributeTypes[key], tftypes.UnknownValue)
			}
			plan := tfsdk.Plan{Schema: sch.Schema, Raw: tftypes.NewValue(ty, planValues)}
			for key, value := range map[string]any{"warehouse_id": "w", "name": "analytics", "cluster_type": "COMPUTE", "node_vcpu": int64(8), "node_count": int64(2), "node_cache": int64(200), "cache_type": int64(0)} {
				if d := plan.SetAttribute(ctx, path.Root(key), value); d.HasError() {
					t.Fatal(d)
				}
			}
			if scenario != "final read failed" && scenario != "creation failed running" && scenario != "running succeeds" {
				if d := plan.SetAttribute(ctx, path.Root("desired_state"), "paused"); d.HasError() {
					t.Fatal(d)
				}
			}
			resp := resource.CreateResponse{State: empty}
			r.Create(ctx, resource.CreateRequest{Plan: plan}, &resp)
			var id types.String
			if d := resp.State.GetAttribute(context.Background(), path.Root("id"), &id); d.HasError() {
				t.Fatal(d)
			}
			t.Logf("recorded id=%q pauses=%d diagnostics=%v", id.ValueString(), pauses, resp.Diagnostics)
			if id.ValueString() != "c-created" {
				t.Error("created cluster identity was lost")
			}
			if (strings.HasPrefix(scenario, "creation failed") || scenario == "create timeout") && pauses != 0 {
				t.Error("pause sent after creation failed")
			}

			if !resp.State.Raw.IsFullyKnown() {
				t.Error("unknown planned values persisted in error state")
			}
			success := strings.HasSuffix(scenario, "succeeds")
			if resp.Diagnostics.HasError() == success {
				t.Errorf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			if success {
				var state ClusterResourceModel
				if d := resp.State.Get(context.Background(), &state); d.HasError() {
					t.Fatal(d)
				}
				wanted := "running"
				if scenario == "paused succeeds" {
					wanted = "paused"
				}
				if state.DesiredState.ValueString() != wanted {
					t.Errorf("desired state=%v", state.DesiredState)
				}
			}

		})
	}
}
