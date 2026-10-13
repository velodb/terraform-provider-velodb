package resource

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestVirtualClusterDeleteConfirmsExactAbsence(t *testing.T) {
	for _, tc := range []struct {
		name, initialStatus, afterDeleteStatus string
		absentAfter, deleteStatus              int
		deleteCode                             string
		readErrorAfter, readStatus             int
		readCode                               string
		malformedAfter                         int
		wantDeletes                            int
		wantError                              bool
	}{
		{name: "delete completes", initialStatus: "Running", absentAfter: 2, wantDeletes: 1},
		{name: "at risk deletion", initialStatus: "AtRisk", absentAfter: 2, wantDeletes: 1},
		{name: "already absent", absentAfter: 1},
		{name: "already deleting", initialStatus: "Deleting", absentAfter: 2},
		{name: "deleted record becomes absent", initialStatus: "Deleted", absentAfter: 2},
		{name: "lost response but absent", initialStatus: "Running", deleteStatus: 500, deleteCode: "InternalError", absentAfter: 2, wantDeletes: 1},
		{name: "lost response but deleting", initialStatus: "Running", deleteStatus: 500, deleteCode: "InternalError", afterDeleteStatus: "Deleting", absentAfter: 3, wantDeletes: 1},
		{name: "concurrent delete", initialStatus: "Running", deleteStatus: 409, deleteCode: "OperationConflict", afterDeleteStatus: "Deleting", absentAfter: 3, wantDeletes: 1},
		{name: "idempotency still processing", initialStatus: "Running", deleteStatus: 202, absentAfter: 3, wantDeletes: 1},
		{name: "delete 404 requires GET confirmation", initialStatus: "Running", deleteStatus: 404, deleteCode: "VirtualClusterNotFound", wantDeletes: 1, wantError: true},
		{name: "delete denied", initialStatus: "Running", deleteStatus: 403, deleteCode: "Forbidden", wantDeletes: 1, wantError: true},
		{name: "warehouse missing", readErrorAfter: 1, readStatus: 404, readCode: "WarehouseNotFound", wantError: true},
		{name: "generic 404", readErrorAfter: 1, readStatus: 404, readCode: "NotFound", wantError: true},
		{name: "read denied", readErrorAfter: 1, readStatus: 403, readCode: "Forbidden", wantError: true},
		{name: "malformed preflight response", malformedAfter: 1, wantError: true},
		{name: "malformed polling response", initialStatus: "Running", malformedAfter: 2, wantDeletes: 1, wantError: true},
		{name: "polling denied", initialStatus: "Running", readErrorAfter: 2, readStatus: 403, readCode: "Forbidden", wantDeletes: 1, wantError: true},
		{name: "deleted status alone is insufficient", initialStatus: "Running", afterDeleteStatus: "Deleted", wantDeletes: 1, wantError: true},
		{name: "existing delete times out", initialStatus: "Deleting", wantError: true},
		{name: "accepted delete remains running", initialStatus: "Running", wantDeletes: 1, wantError: true},
		{name: "updating is busy", initialStatus: "Updating", wantError: true},
		{name: "creating is busy", initialStatus: "Creating", wantError: true},
		{name: "failed create needs backend repair", initialStatus: "CreateFailed", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads, deletes := 0, 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path != "/v1/warehouses/w-1/virtual-clusters/v-1" {
					t.Errorf("deletion must target the ID in prior state: %s", req.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				switch req.Method {
				case http.MethodDelete:
					deletes++
					if req.Header.Get("RequestId") == "" || req.Header.Get("X-API-Key") != "key" {
						t.Error("delete must reuse authentication and write idempotency")
					}
					if tc.deleteStatus != 0 {
						w.WriteHeader(tc.deleteStatus)
						_ = json.NewEncoder(w).Encode(map[string]any{"code": tc.deleteCode, "message": "delete outcome", "requestId": "request-1"})
					} else {
						_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
					}
				case http.MethodGet:
					reads++
					if tc.readErrorAfter > 0 && reads >= tc.readErrorAfter {
						w.WriteHeader(tc.readStatus)
						_ = json.NewEncoder(w).Encode(map[string]any{"code": tc.readCode, "message": "read failed"})
						return
					}
					if tc.absentAfter > 0 && reads >= tc.absentAfter {
						w.WriteHeader(http.StatusNotFound)
						_ = json.NewEncoder(w).Encode(map[string]any{"code": "VirtualClusterNotFound", "message": "target absent"})
						return
					}
					if tc.malformedAfter > 0 && reads >= tc.malformedAfter {
						_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{}})
						return
					}
					status := tc.initialStatus
					if deletes > 0 && tc.afterDeleteStatus != "" {
						status = tc.afterDeleteStatus
					}
					_ = json.NewEncoder(w).Encode(client.APIResponse[client.VirtualCluster]{Success: true, Data: client.VirtualCluster{ID: "v-1", WarehouseID: "w-1", Name: "production", Status: status, ActiveClusterID: "c-a", StandbyClusterID: "c-b"}})
				default:
					t.Errorf("unexpected method: %s", req.Method)
				}
			}))
			defer ts.Close()
			r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
			ctx := context.Background()
			state := virtualClusterTestState(t)
			prior := virtualClusterTestModel()
			prior.Status = types.StringValue("Running")
			prior.Timeouts.Object = types.ObjectValueMust(prior.Timeouts.AttributeTypes(ctx), map[string]attr.Value{
				"create": types.StringNull(), "update": types.StringNull(), "delete": types.StringValue("100ms"),
			})
			if diags := state.Set(ctx, &prior); diags.HasError() {
				t.Fatal(diags)
			}
			resp := resource.DeleteResponse{State: state}
			r.Delete(ctx, resource.DeleteRequest{State: state}, &resp)
			if resp.Diagnostics.HasError() != tc.wantError || deletes != tc.wantDeletes {
				t.Fatalf("unexpected delete result: diagnostics=%v deletes=%d want=%d", resp.Diagnostics, deletes, tc.wantDeletes)
			}
			if tc.wantError {
				if !resp.State.Raw.Equal(state.Raw) {
					t.Fatalf("failed or uncertain deletion must retain prior state: %v", resp.State.Raw)
				}
			} else if !resp.State.Raw.IsNull() {
				t.Fatalf("confirmed target absence must remove state: %v", resp.State.Raw)
			}
		})
	}
}

func TestWaitForVirtualClusterAbsentRequiresExact404(t *testing.T) {
	calls := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := waitForVirtualClusterAbsent(ctx, func(context.Context) (*client.VirtualCluster, error) {
		calls++
		switch calls {
		case 1:
			return &client.VirtualCluster{Status: "Deleting"}, nil
		case 2:
			return &client.VirtualCluster{Status: "Deleted"}, nil
		default:
			return nil, &client.APIError{StatusCode: 404, Code: "VirtualClusterNotFound"}
		}
	}, time.Millisecond)
	if err != nil || calls != 3 {
		t.Fatalf("must wait past Deleted for exact absence: err=%v calls=%d", err, calls)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	err = waitForVirtualClusterAbsent(ctx, func(context.Context) (*client.VirtualCluster, error) {
		return &client.VirtualCluster{Status: "Deleting"}, nil
	}, time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pending delete must time out as an error: %v", err)
	}
}

func TestVirtualClusterDeleteRejectsMissingIdentity(t *testing.T) {
	ctx := context.Background()
	state := virtualClusterTestState(t)
	prior := virtualClusterTestModel()
	prior.ID = types.StringNull()
	if diags := state.Set(ctx, &prior); diags.HasError() {
		t.Fatal(diags)
	}
	r := &VirtualClusterResource{client: client.NewFormationClient("localhost:1", "key", 0, time.Second)}
	resp := resource.DeleteResponse{State: state}
	r.Delete(ctx, resource.DeleteRequest{State: state}, &resp)
	if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) {
		t.Fatalf("missing identity must fail without changing state: %v", resp.Diagnostics)
	}
}

func TestVirtualClusterDeleteRetainsUnresolvedCreateOn404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Errorf("unresolved create must be inspected without another write: %s", req.Method)
		}
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "VirtualClusterNotFound", "message": "hidden or not visible"})
	}))
	defer ts.Close()
	r := &VirtualClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
	ctx := context.Background()
	for _, status := range []types.String{types.StringNull(), types.StringUnknown(), types.StringValue("Creating"), types.StringValue("CreateFailed")} {
		state := virtualClusterTestState(t)
		prior := virtualClusterTestModel()
		prior.Status = status
		if diags := state.Set(ctx, &prior); diags.HasError() {
			t.Fatal(diags)
		}
		resp := resource.DeleteResponse{State: state}
		r.Delete(ctx, resource.DeleteRequest{State: state}, &resp)
		if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) {
			t.Fatalf("unresolved create with status %v must retain state on 404: %v", status, resp.Diagnostics)
		}
	}
}
