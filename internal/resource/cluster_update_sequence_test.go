package resource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestNodeUpdatePreservesCompletedOperations(t *testing.T) {
	for _, scenario := range []string{"rename rejected", "reboot then rename rejected", "auto pause rejected", "reboot rejected", "reboot polling failed", "reboot timeout", "final read failed", "refresh unavailable", "reboot and pause succeed", "rename and pause succeed"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			if scenario == "reboot timeout" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
			}
			count, gets, pauses := 3, 0, 0
			status, name := "Running", "analytics"
			rebooted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				reject := func(code int, message string) {
					w.WriteHeader(code)
					_ = json.NewEncoder(w).Encode(map[string]any{"code": "OperationConflict", "message": message})
				}
				switch {
				case req.Method == http.MethodPatch:
					var update client.UpdateClusterRequest
					if err := json.NewDecoder(req.Body).Decode(&update); err != nil {
						t.Error(err)
					}
					if update.AutoPause != nil && scenario == "auto pause rejected" {
						reject(409, "auto pause rejected")
						return
					}
					if update.NodeCount != nil {
						count = *update.NodeCount
					}
					if update.Name != nil {
						if strings.Contains(scenario, "rename rejected") {
							reject(409, "name unavailable")
							return
						}
						name = *update.Name
					}
					_, _ = w.Write([]byte(`{"success":true}`))
				case strings.HasSuffix(req.URL.Path, "/reboot"):
					if scenario == "reboot rejected" {
						reject(409, "reboot rejected")
						return
					}
					rebooted = true
					status = "Restarting"
					_, _ = w.Write([]byte(`{"success":true}`))
				case strings.HasSuffix(req.URL.Path, "/pause"):
					pauses++
					if status != "Running" {
						reject(409, "cluster is not running")
						return
					}
					status = "Suspended"
					_, _ = w.Write([]byte(`{"success":true}`))
				default:
					gets++
					if scenario == "reboot polling failed" && rebooted && gets == 3 {
						reject(503, "temporary read failure")
						return
					}
					if (scenario == "final read failed" && gets == 3) || (scenario == "refresh unavailable" && gets >= 3) {
						reject(503, "temporary read failure")
						return
					}
					if rebooted && scenario != "reboot polling failed" && scenario != "reboot timeout" && status == "Restarting" {
						status = "Running"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{
						"clusterId": "c", "warehouseId": "w", "name": name, "clusterType": "COMPUTE", "status": status,
						"cacheType": 0, "nodeVcpu": 8, "nodeCount": count, "nodeCache": 200, "capacityModel": "NODE_SPEC",
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
			prior.DesiredState = types.StringValue("running")
			prior.NodeVcpu = types.Int64Value(8)
			prior.NodeCount = types.Int64Value(3)
			prior.NodeCache = types.Int64Value(200)
			prior.CacheType = types.Int64Value(0)
			prior.CapacityModel = types.StringValue("NODE_SPEC")
			if d := priorState.Set(ctx, &prior); d.HasError() {
				t.Fatal(d)
			}
			next := prior
			next.NodeCount = types.Int64Value(4)
			if scenario == "auto pause rejected" {
				next.AutoPause = autoPauseListForTest(types.BoolValue(true), types.Int64Value(15))
			}
			if strings.Contains(scenario, "rename") {
				next.Name = types.StringValue("renamed")
			}
			if strings.Contains(scenario, "reboot") {
				next.RebootTrigger = types.Int64Value(1)
			}
			if strings.Contains(scenario, "pause") || scenario == "reboot polling failed" || scenario == "reboot timeout" {
				next.DesiredState = types.StringValue("paused")
			}
			planState := priorState
			if d := planState.Set(ctx, &next); d.HasError() {
				t.Fatal(d)
			}
			resp := resource.UpdateResponse{State: priorState}
			r.Update(ctx, resource.UpdateRequest{State: priorState, Plan: tfsdk.Plan(planState), Config: tfsdk.Config{Schema: schemaResponse.Schema, Raw: planState.Raw}}, &resp)
			var got ClusterResourceModel
			if d := resp.State.Get(context.Background(), &got); d.HasError() {
				t.Fatal(d)
			}
			t.Logf("remote count=%d, recorded count=%d, pauses=%d, diagnostics=%v", count, got.NodeCount.ValueInt64(), pauses, resp.Diagnostics)
			if (scenario == "reboot polling failed" || scenario == "reboot timeout") && pauses != 0 {
				t.Error("pause sent despite failed reboot completion check")
			}
			if scenario == "reboot polling failed" || scenario == "reboot timeout" {
				if !resp.Diagnostics.HasError() || resp.Diagnostics.Errors()[0].Summary() != "Cluster reboot incomplete" {
					t.Errorf("original reboot failure was not reported: %v", resp.Diagnostics)
				}
			}
			if rebooted && got.RebootTrigger.ValueInt64() != 1 {
				t.Error("accepted reboot trigger lost; retry would reboot again")
			}
			if got.ID.ValueString() != "c" || got.NodeCount.ValueInt64() != int64(count) {
				t.Error("applied node count lost from state after subsequent operation failed")
			}
			success := strings.HasSuffix(scenario, "succeed")
			if resp.Diagnostics.HasError() == success {
				t.Errorf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			if success && (pauses != 1 || got.DesiredState.ValueString() != "paused") {
				t.Error("combined updates did not finish paused")
			}
		})
	}
}
