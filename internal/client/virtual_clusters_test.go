package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestVirtualClusterClientLifecycle(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	c := newTestClient(t, ts)
	base := "/v1/warehouses/w-1/virtual-clusters"
	state := VirtualCluster{
		ID: "v-1", WarehouseID: "w-1", Name: "production", Status: "AtRisk",
		ActiveClusterID: "c-1", StandbyClusterID: "c-2", AtRisk: true,
		RiskReasons: []string{"ComputeResourceLow"},
		CreatedAt:   "2026-10-12T08:00:00Z", UpdatedAt: "2026-10-12T09:00:00Z",
	}
	writeRequest := func(r *http.Request) {
		t.Helper()
		if r.Header.Get("RequestId") == "" {
			t.Error("write request has no RequestId")
		}
		if r.Header.Get("X-API-Key") != "test-api-key" {
			t.Error("write request has no API key")
		}
	}
	mux.HandleFunc(base, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			writeRequest(r)
			var body CreateVirtualClusterRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode create: %v", err)
			}
			if body.Name != "production" || body.ActiveClusterID != "c-1" || body.StandbyClusterID != "c-2" {
				t.Errorf("wrong create body: %+v", body)
			}
			jsonResponse(t, w, http.StatusCreated, APIResponse[CreateVirtualClusterResult]{
				Success: true, Data: CreateVirtualClusterResult{VirtualClusterID: "v-1"},
			})
		case http.MethodGet:
			jsonResponse(t, w, http.StatusOK, APIResponse[[]VirtualCluster]{Success: true, Data: []VirtualCluster{state}})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc(base+"/v-1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			jsonResponse(t, w, http.StatusOK, APIResponse[VirtualCluster]{Success: true, Data: state})
		case http.MethodDelete:
			writeRequest(r)
			jsonResponse(t, w, http.StatusOK, APIResponse[map[string]any]{Success: true, Data: map[string]any{}})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc(base+"/v-1/rename", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodPost) {
			return
		}
		writeRequest(r)
		var body RenameVirtualClusterRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NewName != "renamed" {
			t.Errorf("wrong rename body: %+v, %v", body, err)
		}
		jsonResponse(t, w, http.StatusOK, APIResponse[map[string]any]{Success: true, Data: map[string]any{}})
	})
	mux.HandleFunc(base+"/v-1/update", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodPost) {
			return
		}
		writeRequest(r)
		var body UpdateVirtualClusterRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NewActiveClusterID != "c-2" || body.NewStandbyClusterID != "c-1" {
			t.Errorf("wrong full-pair update body: %+v, %v", body, err)
		}
		jsonResponse(t, w, http.StatusOK, APIResponse[map[string]any]{Success: true, Data: map[string]any{}})
	})

	ctx := context.Background()
	created, err := c.CreateVirtualCluster(ctx, "w-1", &CreateVirtualClusterRequest{
		Name: "production", ActiveClusterID: "c-1", StandbyClusterID: "c-2",
	})
	if err != nil || created == nil || created.VirtualClusterID != "v-1" {
		t.Fatalf("create: %+v, %v", created, err)
	}
	listed, err := c.ListVirtualClusters(ctx, "w-1")
	if err != nil || len(listed) != 1 || listed[0].ID != "v-1" {
		t.Fatalf("list: %+v, %v", listed, err)
	}
	got, err := c.GetVirtualCluster(ctx, "w-1", "v-1")
	if err != nil || got == nil || got.Name != state.Name || got.Status != "AtRisk" || !got.AtRisk ||
		got.ActiveClusterID != "c-1" || got.StandbyClusterID != "c-2" ||
		len(got.RiskReasons) != 1 || got.RiskReasons[0] != "ComputeResourceLow" ||
		got.CreatedAt != state.CreatedAt || got.UpdatedAt != state.UpdatedAt {
		t.Fatalf("get: %+v, %v", got, err)
	}
	if err := c.RenameVirtualCluster(ctx, "w-1", "v-1", &RenameVirtualClusterRequest{NewName: "renamed"}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := c.UpdateVirtualCluster(ctx, "w-1", "v-1", &UpdateVirtualClusterRequest{
		NewActiveClusterID: "c-2", NewStandbyClusterID: "c-1",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := c.DeleteVirtualCluster(ctx, "w-1", "v-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestVirtualClusterNotFoundIsDistinctFromWarehouseNotFound(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	c := newTestClient(t, ts)
	mux.HandleFunc("/v1/warehouses/w-1/virtual-clusters/v-missing", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(t, w, http.StatusNotFound, map[string]any{"code": "VirtualClusterNotFound", "message": "missing"})
	})
	mux.HandleFunc("/v1/warehouses/w-missing/virtual-clusters/v-1", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(t, w, http.StatusNotFound, map[string]any{"code": "WarehouseNotFound", "message": "missing"})
	})
	for _, tc := range []struct {
		warehouseID, virtualClusterID string
		wantTargetMissing             bool
	}{
		{"w-1", "v-missing", true},
		{"w-missing", "v-1", false},
	} {
		_, err := c.GetVirtualCluster(context.Background(), tc.warehouseID, tc.virtualClusterID)
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.IsVirtualClusterNotFound() != tc.wantTargetMissing {
			t.Errorf("GET %s/%s: got %v, want target missing %t", tc.warehouseID, tc.virtualClusterID, err, tc.wantTargetMissing)
		}
	}
}

func TestVirtualClusterClientRejectsMissingIdentity(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	c := newTestClient(t, ts)
	mux.HandleFunc("/v1/warehouses/w-1/virtual-clusters", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			jsonResponse(t, w, http.StatusCreated, APIResponse[map[string]any]{Success: true, Data: map[string]any{}})
			return
		}
		jsonResponse(t, w, http.StatusOK, APIResponse[[]VirtualCluster]{Success: true})
	})
	mux.HandleFunc("/v1/warehouses/w-1/virtual-clusters/v-1", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(t, w, http.StatusOK, APIResponse[VirtualCluster]{Success: true, Data: VirtualCluster{ID: "v-other", WarehouseID: "w-1"}})
	})
	ctx := context.Background()
	if _, err := c.CreateVirtualCluster(ctx, "w-1", &CreateVirtualClusterRequest{Name: "vc", ActiveClusterID: "c-1", StandbyClusterID: "c-2"}); err == nil {
		t.Error("expected error for create response without ID")
	}
	if _, err := c.ListVirtualClusters(ctx, "w-1"); err == nil {
		t.Error("expected error for list response without data array")
	}
	if _, err := c.GetVirtualCluster(ctx, "w-1", "v-1"); err == nil {
		t.Error("expected error for mismatched GET identity")
	}
}
