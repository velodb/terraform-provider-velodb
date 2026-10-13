package resource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func runVirtualClusterUpdate(t *testing.T, r *VirtualClusterResource, prior, planned VirtualClusterResourceModel) (resource.UpdateResponse, VirtualClusterResourceModel) {
	t.Helper()
	ctx := context.Background()
	state, plan := virtualClusterTestState(t), virtualClusterTestState(t)
	planned.ActiveClusterID, planned.StandbyClusterID, planned.Status = types.StringUnknown(), types.StringUnknown(), types.StringUnknown()
	planned.AtRisk, planned.RiskReasons = types.BoolUnknown(), types.SetUnknown(types.StringType)
	planned.CreatedAt, planned.UpdatedAt = types.StringUnknown(), types.StringUnknown()
	if diags := state.Set(ctx, &prior); diags.HasError() {
		t.Fatalf("seed state: %v", diags)
	}
	if diags := plan.Set(ctx, &planned); diags.HasError() {
		t.Fatalf("seed plan: %v", diags)
	}
	resp := resource.UpdateResponse{State: state}
	r.Update(ctx, resource.UpdateRequest{State: state, Plan: tfsdk.Plan(plan)}, &resp)
	var got VirtualClusterResourceModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("decode update state: %v", diags)
	}
	return resp, got
}

func TestVirtualClusterUpdateIntentAndFailover(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		liveActive, liveStandby  string
		newName, active, standby string
		trigger                  bool
		activeWarning            bool
		wantWrites               []string
	}{
		{name: "refresh after failover", liveActive: "c-b", liveStandby: "c-a"},
		{name: "rename after failover", liveActive: "c-b", liveStandby: "c-a", newName: "renamed", wantWrites: []string{"rename"}},
		{name: "replace standby", active: "c-a", standby: "c-c", wantWrites: []string{"update"}},
		{name: "switch roles", active: "c-b", standby: "c-a", activeWarning: true, wantWrites: []string{"update"}},
		{name: "replace both members", active: "c-c", standby: "c-d", activeWarning: true, wantWrites: []string{"update"}},
		{name: "reapply after failover", liveActive: "c-b", liveStandby: "c-a", trigger: true, activeWarning: true, wantWrites: []string{"update"}},
		{name: "replace standby after failover", liveActive: "c-b", liveStandby: "c-a", active: "c-a", standby: "c-c", activeWarning: true, wantWrites: []string{"update"}},
		{name: "reapply matching pair", trigger: true, wantWrites: []string{"update"}},
		{name: "adopt failover", liveActive: "c-b", liveStandby: "c-a", active: "c-b", standby: "c-a"},
		{name: "rename then pair", newName: "renamed", active: "c-a", standby: "c-c", wantWrites: []string{"rename", "update"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prior := virtualClusterTestModel()
			planned := prior
			if tc.newName != "" {
				planned.Name = types.StringValue(tc.newName)
			}
			if tc.active != "" {
				planned.DesiredActiveClusterID, planned.DesiredStandbyClusterID = types.StringValue(tc.active), types.StringValue(tc.standby)
			}
			if tc.trigger {
				planned.PairReconcileTrigger = types.Int64Value(1)
			}
			current := client.VirtualCluster{ID: "v-1", WarehouseID: "w-1", Name: "production", Status: "AtRisk", ActiveClusterID: "c-a", StandbyClusterID: "c-b", AtRisk: true, RiskReasons: []string{"ComputeResourceLow"}}
			if tc.liveActive != "" {
				current.ActiveClusterID, current.StandbyClusterID = tc.liveActive, tc.liveStandby
			}
			var writes []string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.Method == http.MethodGet && req.URL.Path == "/v1/warehouses/w-1/virtual-clusters/v-1" {
					_ = json.NewEncoder(w).Encode(client.APIResponse[client.VirtualCluster]{Success: true, Data: current})
					return
				}
				if req.Method != http.MethodPost || req.Header.Get("RequestId") == "" {
					t.Errorf("unexpected write: %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				switch req.URL.Path {
				case "/v1/warehouses/w-1/virtual-clusters/v-1/rename":
					var body client.RenameVirtualClusterRequest
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.NewName != planned.Name.ValueString() {
						t.Errorf("unexpected rename body: %+v, err=%v", body, err)
					}
					current.Name = body.NewName
					writes = append(writes, "rename")
				case "/v1/warehouses/w-1/virtual-clusters/v-1/update":
					var body client.UpdateVirtualClusterRequest
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.NewActiveClusterID != planned.DesiredActiveClusterID.ValueString() || body.NewStandbyClusterID != planned.DesiredStandbyClusterID.ValueString() {
						t.Errorf("update must send full desired pair: %+v, err=%v", body, err)
					}
					current.ActiveClusterID, current.StandbyClusterID = body.NewActiveClusterID, body.NewStandbyClusterID
					writes = append(writes, "update")
				default:
					t.Errorf("unexpected path: %s", req.URL.Path)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
			}))
			defer ts.Close()
			r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
			resp, got := runVirtualClusterUpdate(t, r, prior, planned)
			if resp.Diagnostics.HasError() || !reflect.DeepEqual(writes, tc.wantWrites) || (resp.Diagnostics.WarningsCount() > 0) != tc.activeWarning {
				t.Fatalf("update: diagnostics=%v writes=%v want=%v", resp.Diagnostics, writes, tc.wantWrites)
			}
			if !got.Name.Equal(planned.Name) || !got.DesiredActiveClusterID.Equal(planned.DesiredActiveClusterID) || !got.DesiredStandbyClusterID.Equal(planned.DesiredStandbyClusterID) || !got.PairReconcileTrigger.Equal(planned.PairReconcileTrigger) || got.ActiveClusterID.ValueString() != current.ActiveClusterID || got.StandbyClusterID.ValueString() != current.StandbyClusterID || !got.AtRisk.ValueBool() {
				t.Fatalf("intent or observed pair lost: %+v", got)
			}
			readResp := resource.ReadResponse{State: resp.State}
			r.Read(context.Background(), resource.ReadRequest{State: resp.State}, &readResp)
			var refreshed VirtualClusterResourceModel
			if diags := readResp.State.Get(context.Background(), &refreshed); diags.HasError() || readResp.Diagnostics.HasError() || !refreshed.DesiredActiveClusterID.Equal(got.DesiredActiveClusterID) || !refreshed.DesiredStandbyClusterID.Equal(got.DesiredStandbyClusterID) || !refreshed.PairReconcileTrigger.Equal(got.PairReconcileTrigger) {
				t.Fatalf("update/read round trip changed intent: %v, %v, %+v", diags, readResp.Diagnostics, refreshed)
			}
		})
	}
}

func TestVirtualClusterUpdateFailurePreservesPartialProgress(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "lost update response", true: "pending timeout"}[pending], func(t *testing.T) {
			current := client.VirtualCluster{ID: "v-1", WarehouseID: "w-1", Name: "production", Status: "Running", ActiveClusterID: "c-a", StandbyClusterID: "c-b"}
			var writes []string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.Method == http.MethodGet {
					_ = json.NewEncoder(w).Encode(client.APIResponse[client.VirtualCluster]{Success: true, Data: current})
					return
				}
				if strings.HasSuffix(req.URL.Path, "/rename") {
					current.Name = "renamed"
					writes = append(writes, "rename")
					_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
					return
				}
				writes = append(writes, "update")
				current.StandbyClusterID = "c-c"
				if pending {
					current.Status = "Updating"
					_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
				} else {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]any{"code": "InternalError", "message": "outcome unknown"})
				}
			}))
			defer ts.Close()
			r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
			prior := virtualClusterTestModel()
			planned := prior
			planned.Name, planned.DesiredStandbyClusterID, planned.PairReconcileTrigger = types.StringValue("renamed"), types.StringValue("c-c"), types.Int64Value(1)
			if pending {
				planned.Timeouts.Object = types.ObjectValueMust(prior.Timeouts.AttributeTypes(context.Background()), map[string]attr.Value{
					"create": types.StringNull(), "update": types.StringValue("100ms"), "delete": types.StringNull(),
				})
			}
			resp, got := runVirtualClusterUpdate(t, r, prior, planned)
			if !resp.Diagnostics.HasError() || got.ID.ValueString() != "v-1" || got.Name.ValueString() != "renamed" || got.StandbyClusterID.ValueString() != "c-c" || !got.DesiredStandbyClusterID.Equal(prior.DesiredStandbyClusterID) || !got.PairReconcileTrigger.Equal(prior.PairReconcileTrigger) || !reflect.DeepEqual(writes, []string{"rename", "update"}) {
				t.Fatalf("failure must preserve ID, verified rename, observed state, and old pair intent: %v, %+v, %v", resp.Diagnostics, got, writes)
			}
		})
	}
}

func TestVirtualClusterUpdateRejectsUnreadyOrStaleName(t *testing.T) {
	for _, tc := range []struct{ status, name string }{{"Updating", "production"}, {"Running", "renamed_elsewhere"}} {
		t.Run(tc.status+"/"+tc.name, func(t *testing.T) {
			writes := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet {
					writes++
				}
				_ = json.NewEncoder(w).Encode(client.APIResponse[client.VirtualCluster]{Success: true, Data: client.VirtualCluster{ID: "v-1", WarehouseID: "w-1", Name: tc.name, Status: tc.status, ActiveClusterID: "c-a", StandbyClusterID: "c-b"}})
			}))
			defer ts.Close()
			r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
			prior := virtualClusterTestModel()
			planned := prior
			planned.DesiredStandbyClusterID = types.StringValue("c-c")
			resp, got := runVirtualClusterUpdate(t, r, prior, planned)
			if !resp.Diagnostics.HasError() || writes != 0 || got.ID.ValueString() != "v-1" || got.Name.ValueString() != tc.name {
				t.Fatalf("unready or stale plan must refresh without a write: %v, %+v, writes=%d", resp.Diagnostics, got, writes)
			}
		})
	}
}

func TestVirtualClusterPlanRejectsWarehouseChangeAndDuplicatePair(t *testing.T) {
	ctx := context.Background()
	r := &VirtualClusterResource{}
	prior := virtualClusterTestModel()
	state := virtualClusterTestState(t)
	if diags := state.Set(ctx, &prior); diags.HasError() {
		t.Fatal(diags)
	}
	for _, warehouseID := range []types.String{types.StringValue("w-2"), types.StringUnknown()} {
		planned := prior
		planned.WarehouseID = warehouseID
		plan := virtualClusterTestState(t)
		if diags := plan.Set(ctx, &planned); diags.HasError() {
			t.Fatal(diags)
		}
		resp := resource.ModifyPlanResponse{Plan: tfsdk.Plan(plan)}
		r.ModifyPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: tfsdk.Plan(plan)}, &resp)
		if !resp.Diagnostics.HasError() {
			t.Fatalf("warehouse change %v must be rejected", warehouseID)
		}
	}
	for _, standbyID := range []types.String{prior.DesiredActiveClusterID, types.StringUnknown()} {
		config := virtualClusterTestState(t)
		planned := prior
		planned.DesiredStandbyClusterID = standbyID
		if diags := config.Set(ctx, &planned); diags.HasError() {
			t.Fatal(diags)
		}
		resp := resource.ValidateConfigResponse{}
		r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config(config)}, &resp)
		if resp.Diagnostics.HasError() == standbyID.IsUnknown() {
			t.Fatalf("reject known duplicate pair but defer unknown values: %v", resp.Diagnostics)
		}
	}
}
