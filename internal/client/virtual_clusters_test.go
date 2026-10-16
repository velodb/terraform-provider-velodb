package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/iotest"
)

func TestCreateVirtualClusterRetriesProcessingWithSameRequest(t *testing.T) {
	c := NewFormationClient("localhost", "test-key", 2, 0)
	var requestID, requestBody string
	var responseBodies []*trackedReadCloser
	calls := 0
	c.HTTPClient.Transport.(*formationTransport).base = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if calls == 1 {
			requestID, requestBody = req.Header.Get("RequestId"), string(body)
		}
		if requestID == "" || req.Header.Get("RequestId") != requestID || string(body) != requestBody {
			t.Fatal("retry must preserve the request ID and body")
		}
		status, payload := http.StatusAccepted, `{"success":true,"data":{"status":"processing"}}`
		if calls == 3 {
			status, payload = http.StatusCreated, `{"success":true,"data":{"virtualClusterId":"v-recovered"}}`
		}
		responseBody := &trackedReadCloser{Reader: strings.NewReader(payload)}
		responseBodies = append(responseBodies, responseBody)
		return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": []string{"0"}}, Body: responseBody, Request: req}, nil
	})
	got, err := c.CreateVirtualCluster(context.Background(), "w-1", &CreateVirtualClusterRequest{Name: "production", ActiveClusterID: "a", StandbyClusterID: "b"})
	if err != nil || got == nil || got.VirtualClusterID != "v-recovered" || calls != 3 {
		t.Fatalf("create recovery: calls=%d result=%+v error=%v", calls, got, err)
	}
	for _, body := range responseBodies {
		if !body.closed {
			t.Fatal("retry and final response bodies must be closed")
		}
	}
}

func TestCreateVirtualClusterErrorsRetainRequestID(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		status        int
		bodyError     bool
		networkError  bool
	}{
		{name: "truncated body", status: http.StatusCreated, bodyError: true},
		{name: "invalid JSON", status: http.StatusCreated, payload: `{"data":`},
		{name: "missing ID", status: http.StatusCreated, payload: `{"success":true,"data":{}}`},
		{name: "processing exhausted", status: http.StatusAccepted, payload: `{"data":{"status":"processing"}}`},
		{name: "API error without request ID", status: http.StatusInternalServerError, payload: `{"code":"InternalError"}`},
		{name: "network error", networkError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewFormationClient("localhost", "test-key", 1, 0)
			requestID, calls := "", 0
			c.HTTPClient.Transport.(*formationTransport).base = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if requestID == "" {
					requestID = req.Header.Get("RequestId")
				}
				if requestID != req.Header.Get("RequestId") {
					t.Fatal("retry changed request ID")
				}
				if tc.networkError {
					return nil, io.ErrUnexpectedEOF
				}
				var body io.Reader = strings.NewReader(tc.payload)
				if tc.bodyError {
					body = iotest.ErrReader(io.ErrUnexpectedEOF)
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Retry-After": []string{"0"}}, Body: io.NopCloser(body), Request: req}, nil
			})
			_, err := c.CreateVirtualCluster(context.Background(), "w-1", &CreateVirtualClusterRequest{Name: "production", ActiveClusterID: "a", StandbyClusterID: "b"})
			if err == nil || requestID == "" || !strings.Contains(err.Error(), requestID) {
				t.Fatalf("uncertain create must report request ID %q: %v", requestID, err)
			}
			if (tc.bodyError || tc.networkError) && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("underlying error lost: %v", err)
			}
			if tc.status == http.StatusAccepted {
				apiErr, ok := err.(*APIError)
				if !ok || apiErr.StatusCode != http.StatusAccepted || calls != 2 || strings.Contains(apiErr.UserMessage(), "Retry after it completes") {
					t.Fatalf("exhausted processing must remain incomplete with safe recovery guidance: calls=%d error=%v", calls, err)
				}
			}
		})
	}
}

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
