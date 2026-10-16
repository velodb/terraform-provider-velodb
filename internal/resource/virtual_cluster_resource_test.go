package resource

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func virtualClusterTestSchema(t *testing.T) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	(&VirtualClusterResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	return resp.Schema
}

func virtualClusterTestState(t *testing.T) tfsdk.State {
	t.Helper()
	s := virtualClusterTestSchema(t)
	return tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(context.Background()), nil)}
}

func virtualClusterTestModel() VirtualClusterResourceModel {
	return VirtualClusterResourceModel{
		ID: types.StringValue("v-1"), WarehouseID: types.StringValue("w-1"), Name: types.StringValue("production"),
		DesiredActiveClusterID: types.StringValue("c-a"), DesiredStandbyClusterID: types.StringValue("c-b"),
		PairReconcileTrigger: types.Int64Null(),
		ActiveClusterID:      types.StringNull(), StandbyClusterID: types.StringNull(), Status: types.StringNull(),
		AtRisk: types.BoolNull(), RiskReasons: types.SetNull(types.StringType),
		CreatedAt: types.StringNull(), UpdatedAt: types.StringNull(),
		Timeouts: timeouts.Value{Object: types.ObjectNull(map[string]attr.Type{
			"create": types.StringType, "update": types.StringType, "delete": types.StringType,
		})},
	}
}

func TestVirtualClusterImportReadAndFailoverDrift(t *testing.T) {
	ctx := context.Background()
	current := client.VirtualCluster{
		ID: "v-1", WarehouseID: "w-1", Name: "production", Status: "Running",
		ActiveClusterID: "c-a", StandbyClusterID: "c-b",
		CreatedAt: "2026-10-12T08:00:00Z", UpdatedAt: "2026-10-12T08:00:00Z",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/warehouses/w-1/virtual-clusters/v-1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(client.APIResponse[client.VirtualCluster]{Success: true, Data: current}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer ts.Close()
	r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
	importResp := resource.ImportStateResponse{State: virtualClusterTestState(t)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "w-1/v-1"}, &importResp)
	if importResp.Diagnostics.HasError() {
		t.Fatalf("import: %v", importResp.Diagnostics)
	}
	read := func(state tfsdk.State) (tfsdk.State, VirtualClusterResourceModel) {
		t.Helper()
		resp := resource.ReadResponse{State: state}
		r.Read(ctx, resource.ReadRequest{State: state}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("read: %v", resp.Diagnostics)
		}
		var model VirtualClusterResourceModel
		if diags := resp.State.Get(ctx, &model); diags.HasError() {
			t.Fatalf("decode state: %v", diags)
		}
		return resp.State, model
	}
	state, first := read(importResp.State)
	if first.ID.ValueString() != "v-1" || first.WarehouseID.ValueString() != "w-1" ||
		first.Name.ValueString() != "production" || first.DesiredActiveClusterID.ValueString() != "c-a" ||
		first.DesiredStandbyClusterID.ValueString() != "c-b" || first.ActiveClusterID.ValueString() != "c-a" ||
		first.StandbyClusterID.ValueString() != "c-b" || first.Status.ValueString() != "Running" ||
		first.CreatedAt.ValueString() != current.CreatedAt || first.UpdatedAt.ValueString() != current.UpdatedAt {
		t.Fatalf("import/read fields: %+v", first)
	}
	if first.RiskReasons.IsNull() || len(first.RiskReasons.Elements()) != 0 {
		t.Fatalf("expected empty risk-reason set: %v", first.RiskReasons)
	}

	current.Name = "renamed_elsewhere"
	current.ActiveClusterID, current.StandbyClusterID = "c-b", "c-a"
	current.Status = "AtRisk"
	current.AtRisk = true
	current.RiskReasons = []string{"ComputeResourceLow", "CacheResourceLow"}
	current.UpdatedAt = "2026-10-12T09:00:00Z"
	state, after := read(state)
	if after.DesiredActiveClusterID.ValueString() != "c-a" || after.DesiredStandbyClusterID.ValueString() != "c-b" ||
		after.ActiveClusterID.ValueString() != "c-b" || after.StandbyClusterID.ValueString() != "c-a" ||
		after.Name.ValueString() != "renamed_elsewhere" || !after.AtRisk.ValueBool() ||
		after.Status.ValueString() != "AtRisk" || after.UpdatedAt.ValueString() != current.UpdatedAt ||
		len(after.RiskReasons.Elements()) != 2 {
		t.Fatalf("failover refresh changed intent or missed observations: %+v", after)
	}
	current.RiskReasons = nil
	current.CreatedAt, current.UpdatedAt = "", ""
	_, omitted := read(state)
	if !omitted.CreatedAt.IsNull() || !omitted.UpdatedAt.IsNull() ||
		omitted.RiskReasons.IsNull() || len(omitted.RiskReasons.Elements()) != 0 {
		t.Fatalf("omitted API values should become null timestamps and an empty risk set: %+v", omitted)
	}
}

func TestVirtualClusterReadOnlyRemovesExactMissingTarget(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		remove     bool
	}{
		{"virtual cluster missing", "VirtualClusterNotFound", true},
		{"warehouse missing", "WarehouseNotFound", false},
		{"permission denied", "Forbidden", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				status := http.StatusNotFound
				if tc.code == "Forbidden" {
					status = http.StatusForbidden
				}
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(map[string]any{"code": tc.code, "message": tc.name})
			}))
			defer ts.Close()
			r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
			state := virtualClusterTestState(t)
			model := virtualClusterTestModel()
			model.Status = types.StringValue("Running")
			if diags := state.Set(ctx, &model); diags.HasError() {
				t.Fatalf("seed state: %v", diags)
			}
			resp := resource.ReadResponse{State: state}
			r.Read(ctx, resource.ReadRequest{State: state}, &resp)
			if tc.remove {
				if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
					t.Fatalf("target missing should remove state: %v, %v", resp.Diagnostics, resp.State.Raw)
				}
			} else if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
				t.Fatalf("other failures must retain state: %v, %v", resp.Diagnostics, resp.State.Raw)
			}
		})
	}
}

func TestVirtualClusterReadRetainsAcceptedCreateOnMissingGET(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "VirtualClusterNotFound", "message": "not visible yet"})
	}))
	defer ts.Close()
	r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
	for _, status := range []types.String{types.StringNull(), types.StringValue("Creating"), types.StringValue("CreateFailed")} {
		ctx := context.Background()
		state := virtualClusterTestState(t)
		model := virtualClusterTestModel()
		model.Status = status
		if diags := state.Set(ctx, &model); diags.HasError() {
			t.Fatalf("seed state: %v", diags)
		}
		resp := resource.ReadResponse{State: state}
		r.Read(ctx, resource.ReadRequest{State: state}, &resp)
		if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
			t.Fatalf("accepted create with status %v must retain state on 404: %v, %v", status, resp.Diagnostics, resp.State.Raw)
		}
	}
}

func TestVirtualClusterImportRejectsInvalidID(t *testing.T) {
	r := &VirtualClusterResource{}
	for _, value := range []string{"", "w-1", "w-1/", "/v-1", "w-1/v-1/extra", " w-1/v-1"} {
		resp := resource.ImportStateResponse{State: virtualClusterTestState(t)}
		r.ImportState(context.Background(), resource.ImportStateRequest{ID: value}, &resp)
		if !resp.Diagnostics.HasError() {
			t.Errorf("expected invalid import ID %q to fail", value)
		}
	}
}

func TestVirtualClusterImportRejectsMissingTarget(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "VirtualClusterNotFound", "message": "missing"})
	}))
	defer ts.Close()
	r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
	resp := resource.ImportStateResponse{State: virtualClusterTestState(t)}
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "w-1/v-1"}, &resp)
	if !resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
		t.Fatalf("missing import target must return an error without state: %v, %v", resp.Diagnostics, resp.State.Raw)
	}
}

func TestVirtualClusterReadCreateFailedRetainsState(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(client.APIResponse[client.VirtualCluster]{
			Success: true,
			Data:    client.VirtualCluster{ID: "v-1", WarehouseID: "w-1", Name: "production", Status: "CreateFailed"},
		})
	}))
	defer ts.Close()
	r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
	ctx := context.Background()
	state := virtualClusterTestState(t)
	model := virtualClusterTestModel()
	if diags := state.Set(ctx, &model); diags.HasError() {
		t.Fatalf("seed state: %v", diags)
	}
	resp := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
		t.Fatalf("CreateFailed must be actionable without dropping state: %v, %v", resp.Diagnostics, resp.State.Raw)
	}
}

func TestVirtualClusterLiveImportRead(t *testing.T) {
	if os.Getenv("VELODB_LIVE_TEST") != "1" {
		t.Skip("set VELODB_LIVE_TEST=1 to run against Formation")
	}
	apiKey := os.Getenv("VELODB_API_KEY")
	warehouseID := os.Getenv("VELODB_TEST_WAREHOUSE_ID")
	virtualClusterID := os.Getenv("VELODB_TEST_VIRTUAL_CLUSTER_ID")
	if apiKey == "" || warehouseID == "" || virtualClusterID == "" {
		t.Fatal("VELODB_API_KEY, VELODB_TEST_WAREHOUSE_ID, and VELODB_TEST_VIRTUAL_CLUSTER_ID are required")
	}
	host := os.Getenv("VELODB_HOST")
	if host == "" {
		host = "api.velodb.cloud"
	}
	r := &VirtualClusterResource{client: client.NewFormationClient(host, apiKey, 5, 60*time.Second)}
	ctx := context.Background()
	importResp := resource.ImportStateResponse{State: virtualClusterTestState(t)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: warehouseID + "/" + virtualClusterID}, &importResp)
	if importResp.Diagnostics.HasError() {
		t.Fatalf("import: %v", importResp.Diagnostics)
	}
	readResp := resource.ReadResponse{State: importResp.State}
	r.Read(ctx, resource.ReadRequest{State: importResp.State}, &readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("read: %v", readResp.Diagnostics)
	}
	var model VirtualClusterResourceModel
	if diags := readResp.State.Get(ctx, &model); diags.HasError() {
		t.Fatalf("decode state: %v", diags)
	}
	if model.ID.ValueString() != virtualClusterID || model.WarehouseID.ValueString() != warehouseID ||
		model.Name.IsNull() || model.Name.ValueString() == "" ||
		model.DesiredActiveClusterID.IsNull() || model.DesiredStandbyClusterID.IsNull() ||
		model.DesiredActiveClusterID != model.ActiveClusterID ||
		model.DesiredStandbyClusterID != model.StandbyClusterID {
		t.Fatalf("import/read did not initialize identity, name, and desired pair from the live cluster")
	}
	t.Logf("import/read succeeded: name=%q status=%q active=%q standby=%q at_risk=%t",
		model.Name.ValueString(), model.Status.ValueString(),
		model.ActiveClusterID.ValueString(), model.StandbyClusterID.ValueString(), model.AtRisk.ValueBool())
}

func TestVirtualClusterCreateKnownIDAndFailureRetainState(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		warning      bool
		processing   bool
	}{
		{name: "running", status: "Running"},
		{name: "at risk", status: "AtRisk"},
		{name: "failed", status: "CreateFailed", warning: true},
		{name: "processing then running", status: "Running", processing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts, requestID := 0, ""
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case http.MethodPost:
					posts++
					if posts == 1 {
						requestID = r.Header.Get("RequestId")
					}
					if requestID == "" || r.Header.Get("RequestId") != requestID {
						t.Error("create retry must preserve the request ID")
					}
					if tc.processing && posts == 1 {
						w.Header().Set("Retry-After", "0")
						w.WriteHeader(http.StatusAccepted)
						_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"status": "processing"}})
						return
					}
					_ = json.NewEncoder(w).Encode(client.APIResponse[client.CreateVirtualClusterResult]{
						Success: true, Data: client.CreateVirtualClusterResult{VirtualClusterID: "v-1"},
					})
				case http.MethodGet:
					_ = json.NewEncoder(w).Encode(client.APIResponse[client.VirtualCluster]{
						Success: true, Data: client.VirtualCluster{
							ID: "v-1", WarehouseID: "w-1", Name: "production", Status: tc.status,
							ActiveClusterID: "c-a", StandbyClusterID: "c-b", AtRisk: tc.status == "AtRisk",
						},
					})
				default:
					t.Errorf("unexpected request method %s", r.Method)
				}
			}))
			defer ts.Close()
			r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 1, time.Second)}
			ctx := context.Background()
			state := virtualClusterTestState(t)
			plan := virtualClusterTestModel()
			plan.ID = types.StringUnknown()
			if diags := state.Set(ctx, &plan); diags.HasError() {
				t.Fatalf("seed plan: %v", diags)
			}
			resp := resource.CreateResponse{State: state}
			r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan(state)}, &resp)
			if resp.Diagnostics.HasError() || resp.Diagnostics.WarningsCount() > 0 != tc.warning {
				t.Fatalf("unexpected create diagnostics: %v", resp.Diagnostics)
			}
			var got VirtualClusterResourceModel
			if diags := resp.State.Get(ctx, &got); diags.HasError() {
				t.Fatalf("decode created state: %v", diags)
			}
			if got.ID.ValueString() != "v-1" || got.Status.ValueString() != tc.status ||
				got.DesiredActiveClusterID.ValueString() != "c-a" || got.ActiveClusterID.ValueString() != "c-a" {
				t.Fatalf("known created cluster must remain in state: %+v", got)
			}
			if tc.processing && posts != 2 {
				t.Fatalf("expected one retry before saving the recovered ID, got %d posts", posts)
			}
		})
	}
}

func TestVirtualClusterCreateWithoutIDReportsUncertainty(t *testing.T) {
	requestID := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID = r.Header.Get("RequestId")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "Unavailable", "message": "retry later"})
	}))
	defer ts.Close()
	r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
	ctx := context.Background()
	state := virtualClusterTestState(t)
	plan := virtualClusterTestModel()
	plan.ID = types.StringUnknown()
	if diags := state.Set(ctx, &plan); diags.HasError() {
		t.Fatalf("seed plan: %v", diags)
	}
	resp := resource.CreateResponse{State: state}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan(state)}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatalf("lost create ID must direct discovery/import before retry: %v", resp.Diagnostics)
	}
	detail := resp.Diagnostics.Errors()[0].Detail()
	for _, expected := range []string{requestID, "production", "w-1", "terraform import <resource_address> w-1/<virtual_cluster_id>", "do not retry creation blindly"} {
		if expected == "" || !strings.Contains(detail, expected) {
			t.Fatalf("create recovery diagnostic missing %q: %s", expected, detail)
		}
	}
}

func TestVirtualClusterCreateKeepsIDWhenReadFails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(client.APIResponse[client.CreateVirtualClusterResult]{
				Success: true, Data: client.CreateVirtualClusterResult{VirtualClusterID: "v-1"},
			})
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "Forbidden", "message": "access denied"})
	}))
	defer ts.Close()
	r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
	ctx := context.Background()
	state := virtualClusterTestState(t)
	plan := virtualClusterTestModel()
	plan.ID = types.StringUnknown()
	if diags := state.Set(ctx, &plan); diags.HasError() {
		t.Fatalf("seed plan: %v", diags)
	}
	resp := resource.CreateResponse{State: state}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan(state)}, &resp)
	var got VirtualClusterResourceModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("decode created state: %v", diags)
	}
	if resp.Diagnostics.HasError() || resp.Diagnostics.WarningsCount() != 1 || got.ID.ValueString() != "v-1" || !got.Status.IsNull() {
		t.Fatalf("known ID must survive a failed follow-up read: diagnostics=%v state=%+v", resp.Diagnostics, got)
	}
}

func TestVirtualClusterCreateRejectsDuplicateMembers(t *testing.T) {
	r := &VirtualClusterResource{client: client.NewFormationClient("localhost:1", "key", 0, time.Second)}
	ctx := context.Background()
	state := virtualClusterTestState(t)
	plan := virtualClusterTestModel()
	plan.ID = types.StringUnknown()
	plan.DesiredStandbyClusterID = plan.DesiredActiveClusterID
	if diags := state.Set(ctx, &plan); diags.HasError() {
		t.Fatalf("seed plan: %v", diags)
	}
	resp := resource.CreateResponse{State: state}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan(state)}, &resp)
	if !resp.Diagnostics.HasError() || resp.Diagnostics.Errors()[0].Summary() != "Invalid virtual-cluster configuration" {
		t.Fatal("duplicate physical-cluster IDs must fail before an API call")
	}
}

func TestWaitForVirtualClusterCreateRequiresFinalStatusAndPair(t *testing.T) {
	calls := 0
	matches := func(cluster *client.VirtualCluster) bool {
		return cluster.ActiveClusterID == "c-a" && cluster.StandbyClusterID == "c-b"
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cluster, err := waitForVirtualCluster(ctx, func(context.Context) (*client.VirtualCluster, error) {
		calls++
		switch calls {
		case 1:
			return nil, &client.APIError{StatusCode: http.StatusNotFound, Code: "VirtualClusterNotFound"}
		case 2:
			return &client.VirtualCluster{Status: "Running", ActiveClusterID: "c-b", StandbyClusterID: "c-a"}, nil
		default:
			return &client.VirtualCluster{Status: "Running", ActiveClusterID: "c-a", StandbyClusterID: "c-b"}, nil
		}
	}, matches, true, time.Millisecond)
	if err != nil || cluster == nil || calls != 3 {
		t.Fatalf("wait returned before the requested pair was Running: cluster=%+v err=%v calls=%d", cluster, err, calls)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Millisecond)
	defer cancel()
	cluster, err = waitForVirtualCluster(ctx, func(context.Context) (*client.VirtualCluster, error) {
		return &client.VirtualCluster{Status: "Creating"}, nil
	}, matches, true, time.Millisecond)
	if cluster == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pending create should return last observation and deadline: cluster=%+v err=%v", cluster, err)
	}
}
