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

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestVirtualClusterReadRoleSwitchWarning(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		priorActive, priorStandby types.String
		active, standby           string
		warning                   bool
	}{
		{"roles reversed", types.StringValue("c-a"), types.StringValue("c-b"), "c-b", "c-a", true},
		{"roles unchanged", types.StringValue("c-a"), types.StringValue("c-b"), "c-a", "c-b", false},
		{"member replaced", types.StringValue("c-a"), types.StringValue("c-b"), "c-a", "c-c", false},
		{"first observation", types.StringNull(), types.StringNull(), "c-b", "c-a", false},
		{"unknown prior roles", types.StringUnknown(), types.StringUnknown(), "c-b", "c-a", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet {
					t.Errorf("refresh must not write: %s", req.Method)
				}
				_ = json.NewEncoder(w).Encode(client.APIResponse[client.VirtualCluster]{Success: true, Data: client.VirtualCluster{
					ID: "v-1", WarehouseID: "w-1", Name: "production", Status: "Running", ActiveClusterID: tc.active, StandbyClusterID: tc.standby,
				}})
			}))
			defer ts.Close()
			r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
			state := virtualClusterTestState(t)
			prior := virtualClusterTestModel()
			prior.ActiveClusterID, prior.StandbyClusterID = tc.priorActive, tc.priorStandby
			if diags := state.Set(ctx, &prior); diags.HasError() {
				t.Fatal(diags)
			}
			resp := resource.ReadResponse{State: state}
			r.Read(ctx, resource.ReadRequest{State: state}, &resp)
			if resp.Diagnostics.HasError() || (resp.Diagnostics.WarningsCount() > 0) != tc.warning {
				t.Fatalf("unexpected refresh diagnostics: %v", resp.Diagnostics)
			}
			if tc.warning && (!strings.Contains(resp.Diagnostics.Warnings()[0].Detail(), `Active: "c-a" -> "c-b"`) || !strings.Contains(resp.Diagnostics.Warnings()[0].Detail(), "configured desired pair is unchanged")) {
				t.Fatalf("warning must explain the switch and preserved intent: %v", resp.Diagnostics)
			}
			var got VirtualClusterResourceModel
			if diags := resp.State.Get(ctx, &got); diags.HasError() || got.ActiveClusterID.ValueString() != tc.active || got.StandbyClusterID.ValueString() != tc.standby || !got.DesiredActiveClusterID.Equal(prior.DesiredActiveClusterID) || !got.DesiredStandbyClusterID.Equal(prior.DesiredStandbyClusterID) {
				t.Fatalf("warning must preserve intent and refresh roles: %v, %+v", diags, got)
			}
			second := resource.ReadResponse{State: resp.State}
			r.Read(ctx, resource.ReadRequest{State: resp.State}, &second)
			if second.Diagnostics.HasError() || second.Diagnostics.WarningsCount() != 0 {
				t.Fatalf("unchanged roles should not warn after refreshed state is persisted: %v", second.Diagnostics)
			}
		})
	}
}

func TestVirtualClusterPlanActiveChangeWarning(t *testing.T) {
	for _, tc := range []struct {
		name            string
		active, standby types.String
		trigger         types.Int64
		warningContains string
	}{
		{"unchanged desired pair after failover", types.StringValue("c-a"), types.StringValue("c-b"), types.Int64Null(), ""},
		{"standby edit would restore old active", types.StringValue("c-a"), types.StringValue("c-c"), types.Int64Null(), `from "c-b" to "c-a"`},
		{"explicit reconciliation", types.StringValue("c-a"), types.StringValue("c-b"), types.Int64Value(1), `from "c-b" to "c-a"`},
		{"adopt live roles", types.StringValue("c-b"), types.StringValue("c-a"), types.Int64Null(), ""},
		{"replace active", types.StringValue("c-c"), types.StringValue("c-a"), types.Int64Null(), `from "c-b" to "c-c"`},
		{"unknown active target", types.StringUnknown(), types.StringValue("c-c"), types.Int64Null(), "requested active cluster ID is not yet known"},
		{"unknown standby target", types.StringValue("c-a"), types.StringUnknown(), types.Int64Null(), "(known after apply)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			prior := virtualClusterTestModel()
			prior.ActiveClusterID, prior.StandbyClusterID = types.StringValue("c-b"), types.StringValue("c-a")
			planned := prior
			planned.Name = types.StringValue("renamed")
			planned.DesiredActiveClusterID, planned.DesiredStandbyClusterID, planned.PairReconcileTrigger = tc.active, tc.standby, tc.trigger
			state, plan := virtualClusterTestState(t), virtualClusterTestState(t)
			if diags := state.Set(ctx, &prior); diags.HasError() {
				t.Fatal(diags)
			}
			if diags := plan.Set(ctx, &planned); diags.HasError() {
				t.Fatal(diags)
			}
			resp := resource.ModifyPlanResponse{Plan: tfsdk.Plan(plan)}
			(&VirtualClusterResource{}).ModifyPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: tfsdk.Plan(plan)}, &resp)
			if resp.Diagnostics.HasError() || (resp.Diagnostics.WarningsCount() > 0) != (tc.warningContains != "") {
				t.Fatalf("unexpected plan diagnostics: %v", resp.Diagnostics)
			}
			if tc.warningContains != "" && !strings.Contains(resp.Diagnostics.Warnings()[0].Detail(), tc.warningContains) {
				t.Fatalf("warning must describe the actual target: %v", resp.Diagnostics)
			}
		})
	}
}

func TestVirtualClusterPlanDestroyWarning(t *testing.T) {
	ctx := context.Background()
	state := virtualClusterTestState(t)
	prior := virtualClusterTestModel()
	if diags := state.Set(ctx, &prior); diags.HasError() {
		t.Fatal(diags)
	}
	plan := tfsdk.Plan(virtualClusterTestState(t))
	resp := resource.ModifyPlanResponse{Plan: plan}
	(&VirtualClusterResource{}).ModifyPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: plan}, &resp)
	if resp.Diagnostics.HasError() || resp.Diagnostics.WarningsCount() != 1 {
		t.Fatalf("destroy must warn without blocking: %v", resp.Diagnostics)
	}
	for _, want := range []string{`"production"`, `"v-1"`, "endpoint", "failover", "-target", "prevent_destroy", "-auto-approve"} {
		if !strings.Contains(resp.Diagnostics.Warnings()[0].Detail(), want) {
			t.Errorf("destroy warning must include %q: %v", want, resp.Diagnostics)
		}
	}
	if !resp.Plan.Raw.IsNull() {
		t.Fatal("warning must preserve the destroy plan")
	}
}
