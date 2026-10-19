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
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func warehouseIdentityTestState(t *testing.T) tfsdk.State {
	t.Helper()
	var resp resource.SchemaResponse
	(&WarehouseResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	ty := resp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
	values := map[string]tftypes.Value{}
	for name, attrType := range ty.AttributeTypes {
		values[name] = tftypes.NewValue(attrType, nil)
	}
	return tfsdk.State{Schema: resp.Schema, Raw: tftypes.NewValue(ty, values)}
}

func TestWarehouseCreateCapturesOnlyNewWarehouseIdentity(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(map[bool]string{false: "discovered", true: "discovery denied"}[denied], func(t *testing.T) {
			lists, creates := 0, 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch {
				case req.Method == http.MethodPost && req.URL.Path == "/v1/warehouses":
					creates++
					_ = json.NewEncoder(w).Encode(client.APIResponse[client.CreateWarehouseResult]{Success: true, Data: client.CreateWarehouseResult{WarehouseID: "w-new"}})
				case req.Method == http.MethodGet && req.URL.Path == "/v1/warehouses/w-new":
					_ = json.NewEncoder(w).Encode(client.APIResponse[client.WarehouseItem]{Success: true, Data: client.WarehouseItem{WarehouseID: "w-new", Status: "Running"}})
				case req.Method == http.MethodGet && req.URL.Path == "/v1/warehouses/w-new/clusters":
					lists++
					if denied {
						w.WriteHeader(http.StatusForbidden)
						_ = json.NewEncoder(w).Encode(map[string]any{"code": "Forbidden", "message": "list denied", "requestId": "request-test"})
						return
					}
					_ = json.NewEncoder(w).Encode(client.PageResponse[client.ClusterItem]{Success: true, Data: []client.ClusterItem{{ClusterID: "c-new", WarehouseID: "w-new", ClusterType: "COMPUTE"}}, Total: 1})
				default:
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer ts.Close()
			ctx := context.Background()
			state := warehouseIdentityTestState(t)
			plan := tfsdk.Plan(state)
			for name, value := range map[string]any{
				"id": "w-old", "initial_cluster_id": "c-old", "name": "test", "deployment_mode": "SaaS",
				"cloud_provider": "aws", "region": "us-east-1", "admin_password": "TestPass@123",
				"initial_cluster": initialClusterListForTest(types.Int64Null()),
			} {
				if diags := plan.SetAttribute(ctx, path.Root(name), value); diags.HasError() {
					t.Fatal(diags)
				}
			}
			r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
			resp := resource.CreateResponse{State: state}
			r.Create(ctx, resource.CreateRequest{Plan: plan}, &resp)
			var result WarehouseResourceModel
			if diags := resp.State.Get(ctx, &result); diags.HasError() {
				t.Fatal(diags)
			}
			want := types.StringValue("c-new")
			if denied {
				want = types.StringNull()
			}
			if resp.Diagnostics.HasError() || result.ID.ValueString() != "w-new" || !result.InitialClusterID.Equal(want) || lists != 1 || creates != 1 {
				t.Fatalf("created warehouse ID must survive discovery and old identity must not leak: result=%+v diagnostics=%v lists=%d creates=%d", result.ID, resp.Diagnostics, lists, creates)
			}
			if (len(resp.Diagnostics.Warnings()) > 0) != denied {
				t.Fatalf("failed discovery must warn: %v", resp.Diagnostics)
			}
			if denied && !strings.Contains(resp.Diagnostics.Warnings()[0].Detail(), "request-test") {
				t.Fatalf("warning must retain API request ID: %v", resp.Diagnostics)
			}
			read := resource.ReadResponse{State: resp.State}
			r.Read(ctx, resource.ReadRequest{State: resp.State}, &read)
			if read.Diagnostics.HasError() || lists != 1 {
				t.Fatalf("refresh must not repeat creation discovery: %v lists=%d", read.Diagnostics, lists)
			}
		})
	}
}

func TestWarehouseImportReadUpdateLeaveInitialIdentityNull(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/warehouses/w-existing" {
			t.Errorf("import/read/update must not infer identity from clusters or mutate the warehouse: %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(client.APIResponse[client.WarehouseItem]{Success: true, Data: client.WarehouseItem{WarehouseID: "w-existing", Name: "existing", Status: "Running"}})
	}))
	defer ts.Close()
	ctx := context.Background()
	r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
	imported := resource.ImportStateResponse{State: warehouseIdentityTestState(t)}
	if diags := imported.State.SetAttribute(ctx, path.Root("initial_cluster_id"), "stale-cached-member"); diags.HasError() {
		t.Fatal(diags)
	}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "w-existing"}, &imported)
	if imported.Diagnostics.HasError() || len(imported.Diagnostics.Warnings()) != 1 {
		t.Fatalf("import must succeed with separate-cluster import guidance: %v", imported.Diagnostics)
	}
	if !strings.Contains(imported.Diagnostics.Warnings()[0].Detail(), "velodb_cluster") {
		t.Fatalf("import guidance must explain how to manage ordinary physical clusters: %v", imported.Diagnostics)
	}
	var importedWarehouse WarehouseResourceModel
	if diags := imported.State.Get(ctx, &importedWarehouse); diags.HasError() {
		t.Fatal(diags)
	}
	if !importedWarehouse.InitialClusterID.IsNull() || !importedWarehouse.InitialCluster.IsNull() {
		t.Fatalf("import must not assign a cached member or reconstruct initial-cluster creation settings: %+v", importedWarehouse)
	}
	read := resource.ReadResponse{State: imported.State}
	r.Read(ctx, resource.ReadRequest{State: imported.State}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	plan := tfsdk.Plan(read.State)
	if diags := plan.SetAttribute(ctx, path.Root("admin_password_version"), int64(1)); diags.HasError() {
		t.Fatal(diags)
	}
	updated := resource.UpdateResponse{State: read.State}
	r.Update(ctx, resource.UpdateRequest{Config: tfsdk.Config(plan), Plan: plan, State: read.State}, &updated)
	var result WarehouseResourceModel
	if diags := updated.State.Get(ctx, &result); diags.HasError() {
		t.Fatal(diags)
	}
	if updated.Diagnostics.HasError() || result.ID.ValueString() != "w-existing" || !result.InitialClusterID.IsNull() || requests != 3 {
		t.Fatalf("imported identity must remain unresolved through Read and Update: id=%s diagnostics=%v requests=%d", result.InitialClusterID, updated.Diagnostics, requests)
	}
}

func TestWarehouseReadDoesNotInferInitialCluster(t *testing.T) {
	for _, prior := range []types.String{types.StringNull(), types.StringUnknown(), types.StringValue("cached-original"), types.StringValue("legacy-unverified")} {
		t.Run(prior.String(), func(t *testing.T) {
			reads := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				reads++
				if req.Method != http.MethodGet || req.URL.Path != "/v1/warehouses/w-test" {
					t.Errorf("refresh must not list or mutate physical clusters: %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				_ = json.NewEncoder(w).Encode(client.APIResponse[client.WarehouseItem]{Success: true, Data: client.WarehouseItem{WarehouseID: "w-test"}})
			}))
			defer ts.Close()
			r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
			state := WarehouseResourceModel{InitialClusterID: prior}
			want := prior
			if prior.IsUnknown() {
				want = types.StringNull()
			}
			for range 3 {
				var diagnostics diag.Diagnostics
				r.readWarehouseIntoState(context.Background(), "w-test", &state, &diagnostics)
				if diagnostics.HasError() || !state.InitialClusterID.Equal(want) {
					t.Fatalf("refresh must preserve cached identity or remain null: id=%s diagnostics=%v", state.InitialClusterID, diagnostics)
				}
			}
			if reads != 3 {
				t.Fatalf("expected warehouse reads only, got %d requests", reads)
			}
		})
	}
}

func TestCreatedWarehouseInitialClusterDiscovery(t *testing.T) {
	compute := client.ClusterItem{ClusterID: "c-original", WarehouseID: "w-new", ClusterType: "COMPUTE"}
	other := client.ClusterItem{ClusterID: "c-other", WarehouseID: "w-new", ClusterType: "COMPUTE"}
	observer := client.ClusterItem{ClusterID: "observer", WarehouseID: "w-new", ClusterType: "OBSERVER"}
	system := client.ClusterItem{ClusterID: "m-system", WarehouseID: "w-new", ClusterType: "COMPUTE"}
	for _, tc := range []struct {
		name     string
		clusters []client.ClusterItem
		total    int64
		status   int
		wantID   string
	}{
		{name: "only compute", clusters: []client.ClusterItem{compute}, wantID: compute.ClusterID},
		{name: "observer and system first", clusters: []client.ClusterItem{observer, system, compute}, wantID: compute.ClusterID},
		{name: "ambiguous newest first", clusters: []client.ClusterItem{other, compute}},
		{name: "ambiguous oldest first", clusters: []client.ClusterItem{compute, other}},
		{name: "no compute", clusters: []client.ClusterItem{observer, system}},
		{name: "not yet visible"},
		{name: "list denied", status: http.StatusForbidden},
		{name: "incomplete page", clusters: []client.ClusterItem{compute}, total: 51},
		{name: "full page without total", clusters: make([]client.ClusterItem, 50)},
		{name: "missing cluster ID", clusters: []client.ClusterItem{{WarehouseID: "w-new", ClusterType: "COMPUTE"}}},
		{name: "wrong warehouse", clusters: []client.ClusterItem{{ClusterID: "c-old", WarehouseID: "w-old", ClusterType: "COMPUTE"}}},
		{name: "missing type", clusters: []client.ClusterItem{{ClusterID: "c-unknown", WarehouseID: "w-new"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls++
				if req.Method != http.MethodGet || req.URL.Path != "/v1/warehouses/w-new/clusters" || req.URL.Query().Get("page") != "1" || req.URL.Query().Get("size") != "50" {
					t.Errorf("discovery must target the newly created warehouse: %s %s", req.Method, req.URL)
				}
				if tc.status != 0 {
					w.WriteHeader(tc.status)
					_ = json.NewEncoder(w).Encode(map[string]any{"code": "Forbidden", "message": "list denied", "requestId": "request-test"})
					return
				}
				_ = json.NewEncoder(w).Encode(client.PageResponse[client.ClusterItem]{Success: true, Data: tc.clusters, Total: tc.total})
			}))
			defer ts.Close()
			r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
			id, err := r.createdWarehouseInitialClusterID(context.Background(), "w-new")
			if id != tc.wantID || (err != nil) != (tc.wantID == "") || calls != 1 {
				t.Fatalf("id=%q error=%v calls=%d; want id=%q", id, err, calls, tc.wantID)
			}
		})
	}
}
