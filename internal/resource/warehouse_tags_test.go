package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestWarehouseTagsLifecycle(t *testing.T) {
	ctx := context.Background()
	source := map[string]string{}
	creates, patches := 0, 0
	failUpdate := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/warehouses":
			creates++
			var input client.CreateWarehouseRequest
			if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
				t.Error(err)
			}
			source = input.Tags
			_, _ = fmt.Fprint(w, `{"success":true,"data":{"warehouseId":"w-tags"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/warehouses/w-tags":
			patches++
			if failUpdate {
				w.WriteHeader(http.StatusConflict)
				_, _ = fmt.Fprint(w, `{"success":false,"code":"OperationConflict","message":"retry later"}`)
				return
			}
			var input client.UpdateWarehouseRequest
			if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
				t.Error(err)
			}
			if input.Tags == nil || *input.Tags == nil || input.Name != nil {
				t.Errorf("tag-only update must send a map, including {} for removal: %+v", input)
			} else {
				source = *input.Tags
			}
			_, _ = fmt.Fprint(w, `{"success":true}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/warehouses/w-tags":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{
				"warehouseId": "w-tags", "name": "analytics", "status": "Running",
				"cloudProvider": "aws", "region": "us-east-1", "deploymentMode": "BYOC", "tags": source,
			}})
		case req.Method == http.MethodGet && req.URL.Path == "/v1/warehouses/w-tags/clusters":
			_, _ = fmt.Fprint(w, `{"success":true,"total":1,"data":[{"clusterId":"c-tags","warehouseId":"w-tags","clusterType":"COMPUTE"}]}`)
		default:
			t.Errorf("unexpected operation: %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "test", 0, time.Second)}
	state := warehouseIdentityTestState(t)
	plan := tfsdk.Plan(state)
	initialTags := map[string]string{"team": "analytics", "empty": ""}
	for name, value := range map[string]any{
		"name": "analytics", "deployment_mode": "BYOC", "cloud_provider": "aws", "region": "us-east-1",
		"setup_mode": "advanced", "credential_id": int64(123), "network_config_id": int64(456),
		"admin_password": "TestPass@123", "initial_cluster": initialClusterListForTest(types.Int64Null()), "tags": initialTags,
	} {
		if diags := plan.SetAttribute(ctx, path.Root(name), value); diags.HasError() {
			t.Fatal(diags)
		}
	}
	created := resource.CreateResponse{State: state}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, &created)
	if created.Diagnostics.HasError() || creates != 1 {
		t.Fatalf("create: %v, requests=%d", created.Diagnostics, creates)
	}
	state = created.State
	checkWarehouseTags(t, state, initialTags)

	for _, next := range []map[string]string{
		{"team": "analytics", "empty": "", "env": "test"},
		{"team": "platform", "env": "production"},
		{},
		{"team": "analytics"},
		nil,
	} {
		plan = tfsdk.Plan(state)
		if diags := plan.SetAttribute(ctx, path.Root("tags"), next); diags.HasError() {
			t.Fatal(diags)
		}
		modified := resource.ModifyPlanResponse{Plan: plan}
		r.ModifyPlan(ctx, resource.ModifyPlanRequest{Config: tfsdk.Config(plan), Plan: plan, State: state}, &modified)
		if modified.Diagnostics.HasError() || len(modified.RequiresReplace) != 0 {
			t.Fatalf("tags must update in place: %v, replacement=%v", modified.Diagnostics, modified.RequiresReplace)
		}
		before := patches
		updated := resource.UpdateResponse{State: state}
		r.Update(ctx, resource.UpdateRequest{Config: tfsdk.Config(plan), Plan: plan, State: state}, &updated)
		if updated.Diagnostics.HasError() || patches != before+1 {
			t.Fatalf("update: %v, requests=%d", updated.Diagnostics, patches-before)
		}
		state = updated.State
		checkWarehouseTags(t, state, next)
		unchanged := resource.UpdateResponse{State: state}
		plan = tfsdk.Plan(state)
		r.Update(ctx, resource.UpdateRequest{Config: tfsdk.Config(plan), Plan: plan, State: state}, &unchanged)
		if unchanged.Diagnostics.HasError() || patches != before+1 {
			t.Fatalf("unchanged tags must not PATCH again: %v", unchanged.Diagnostics)
		}
	}

	source = map[string]string{"team": "changed-outside-terraform"}
	read := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	state = read.State
	checkWarehouseTags(t, state, source)
	imported := resource.ImportStateResponse{State: warehouseIdentityTestState(t)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "w-tags"}, &imported)
	read = resource.ReadResponse{State: imported.State}
	r.Read(ctx, resource.ReadRequest{State: imported.State}, &read)
	if imported.Diagnostics.HasError() || read.Diagnostics.HasError() {
		t.Fatalf("import/read: %v %v", imported.Diagnostics, read.Diagnostics)
	}
	checkWarehouseTags(t, read.State, source)

	failUpdate = true
	plan = tfsdk.Plan(state)
	if diags := plan.SetAttribute(ctx, path.Root("tags"), map[string]string{}); diags.HasError() {
		t.Fatal(diags)
	}
	failed := resource.UpdateResponse{State: state}
	r.Update(ctx, resource.UpdateRequest{Config: tfsdk.Config(plan), Plan: plan, State: state}, &failed)
	if !failed.Diagnostics.HasError() || creates != 1 {
		t.Fatalf("failed update must report failure without recreating: %v, creates=%d", failed.Diagnostics, creates)
	}
	checkWarehouseTags(t, failed.State, source)
}

func checkWarehouseTags(t *testing.T, state tfsdk.State, want map[string]string) {
	t.Helper()
	var got map[string]string
	if diags := state.GetAttribute(context.Background(), path.Root("tags"), &got); diags.HasError() {
		t.Fatal(diags)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %#v, want %#v", got, want)
	}
}

func TestWarehouseTagReadbackCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		prior       types.Map
		want        map[string]string
	}{
		{"empty clears prior values", `,"tags":{}`, types.MapValueMust(types.StringType, map[string]attr.Value{"team": types.StringValue("old")}), map[string]string{}},
		{"empty keeps unconfigured null", `,"tags":{}`, types.MapNull(types.StringType), nil},
		{"omitted retains known state", "", types.MapValueMust(types.StringType, map[string]attr.Value{"team": types.StringValue("old")}), map[string]string{"team": "old"}},
		{"null retains known state", `,"tags":null`, types.MapValueMust(types.StringType, map[string]attr.Value{"team": types.StringValue("old")}), map[string]string{"team": "old"}},
		{"omitted import stays null", "", types.MapNull(types.StringType), nil},
		{"omitted unknown resolves null", "", types.MapUnknown(types.StringType), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"success":true,"data":{"warehouseId":"w-tags"%s}}`, tc.field)
			}))
			defer server.Close()
			r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "test", 0, time.Second)}
			state := WarehouseResourceModel{Tags: tc.prior}
			var diagnostics diag.Diagnostics
			r.readWarehouseIntoState(context.Background(), "w-tags", &state, &diagnostics)
			var got map[string]string
			diagnostics.Append(state.Tags.ElementsAs(context.Background(), &got, false)...)
			if diagnostics.HasError() || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("tags = %#v, want %#v; diagnostics=%v", got, tc.want, diagnostics)
			}
		})
	}
}
