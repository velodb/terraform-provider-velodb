package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateCluster(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001/clusters", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodPost) {
			return
		}
		if r.Header.Get("RequestId") == "" {
			t.Error("expected RequestId header for POST")
		}

		var req CreateClusterRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Name != "compute-etl" {
			t.Errorf("expected name 'compute-etl', got %q", req.Name)
		}
		if req.ClusterType != "COMPUTE" {
			t.Errorf("expected clusterType 'COMPUTE', got %q", req.ClusterType)
		}
		if req.ComputeVcpu != 4 {
			t.Errorf("expected computeVcpu 4, got %d", req.ComputeVcpu)
		}
		if req.CacheGb != 100 {
			t.Errorf("expected cacheGb 100, got %d", req.CacheGb)
		}
		if req.AutoPause == nil || req.AutoPause.Enabled != false {
			t.Error("expected autoPause.enabled=false")
		}

		jsonResponse(w, 200, APIResponse[CreateClusterResult]{
			Success:   true,
			RequestID: "req-020",
			Data: CreateClusterResult{
				ClusterID: "CL-001",
			},
		})
	})

	zone := "cn-beijing-k"
	timeout := 50
	result, err := client.CreateCluster(context.Background(), "WH-001", &CreateClusterRequest{
		Name:        "compute-etl",
		ClusterType: "COMPUTE",
		Zone:        &zone,
		ComputeVcpu: 4,
		CacheGb:     100,
		AutoPause: &AutoPauseConfig{
			Enabled:            false,
			IdleTimeoutMinutes: &timeout,
		},
	})
	if err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}
	if result.ClusterID != "CL-001" {
		t.Errorf("expected 'CL-001', got %q", result.ClusterID)
	}
}

func TestGetCluster(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001/clusters/CL-001", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodGet) {
			return
		}
		jsonResponse(w, 200, APIResponse[ClusterItem]{
			Success:   true,
			RequestID: "req-021",
			Data:      mockCluster("CL-001", "WH-001", "compute-etl"),
		})
	})

	cl, err := client.GetCluster(context.Background(), "WH-001", "CL-001")
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if cl.ClusterID != "CL-001" {
		t.Errorf("expected 'CL-001', got %q", cl.ClusterID)
	}
	if cl.Status != "Running" {
		t.Errorf("expected 'Running', got %q", cl.Status)
	}
	if cl.ConnectionInfo == nil {
		t.Fatal("expected connectionInfo")
	}
	if cl.ConnectionInfo.ListenerPort != 9030 {
		t.Errorf("expected listenerPort 9030, got %d", cl.ConnectionInfo.ListenerPort)
	}
}

func TestGetClusterNotFound(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001/clusters/CL-MISSING", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 404, map[string]any{
			"code":      "ClusterNotFound",
			"message":   "The cluster [CL-MISSING] not found",
			"success":   false,
			"requestId": "req-022",
		})
	})

	_, err := client.GetCluster(context.Background(), "WH-001", "CL-MISSING")
	if err == nil {
		t.Fatal("expected error for missing cluster")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !apiErr.IsNotFound() {
		t.Error("expected IsNotFound=true")
	}
}

func TestListClusters(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001/clusters", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodGet) {
			return
		}

		q := r.URL.Query()
		if q.Get("clusterType") != "COMPUTE" {
			t.Errorf("expected clusterType=COMPUTE, got %q", q.Get("clusterType"))
		}
		if q.Get("status") != "Running" {
			t.Errorf("expected status=Running, got %q", q.Get("status"))
		}

		jsonResponse(w, 200, PageResponse[ClusterItem]{
			Success:   true,
			RequestID: "req-023",
			Data: []ClusterItem{
				mockCluster("CL-001", "WH-001", "compute-etl"),
				mockCluster("CL-002", "WH-001", "compute-dev"),
			},
			Page:  1,
			Size:  20,
			Total: 2,
		})
	})

	result, err := client.ListClusters(context.Background(), "WH-001", &ListClustersOptions{
		Page:        1,
		Size:        20,
		Status:      "Running",
		ClusterType: "COMPUTE",
	})
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected total=2, got %d", result.Total)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(result.Data))
	}
}

func TestUpdateCluster(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001/clusters/CL-001", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodPatch) {
			return
		}
		var req UpdateClusterRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Name == nil || *req.Name != "compute-renamed" {
			t.Error("expected name 'compute-renamed'")
		}
		if req.ComputeVcpu == nil || *req.ComputeVcpu != 8 {
			t.Error("expected computeVcpu 8")
		}
		if req.CacheGb == nil || *req.CacheGb != 500 {
			t.Error("expected cacheGb 500")
		}
		if req.AutoPause == nil || !req.AutoPause.Enabled {
			t.Error("expected autoPause.enabled=true")
		}

		jsonResponse(w, 200, APIResponse[struct{}]{
			Success:   true,
			RequestID: "req-024",
		})
	})

	name := "compute-renamed"
	vcpu := 8
	cache := 500
	timeout := 10
	err := client.UpdateCluster(context.Background(), "WH-001", "CL-001", &UpdateClusterRequest{
		Name:        &name,
		ComputeVcpu: &vcpu,
		CacheGb:     &cache,
		AutoPause: &AutoPauseConfig{
			Enabled:            true,
			IdleTimeoutMinutes: &timeout,
		},
	})
	if err != nil {
		t.Fatalf("UpdateCluster: %v", err)
	}
}

func TestDeleteCluster(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001/clusters/CL-001", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodDelete) {
			return
		}
		jsonResponse(w, 200, APIResponse[struct{}]{
			Success:   true,
			RequestID: "req-025",
		})
	})

	err := client.DeleteCluster(context.Background(), "WH-001", "CL-001")
	if err != nil {
		t.Fatalf("DeleteCluster: %v", err)
	}
}

func TestClusterActions(t *testing.T) {
	for _, action := range []string{"pause", "resume", "reboot"} {
		t.Run(action, func(t *testing.T) {
			ts, mux := newTestServer(t)
			defer ts.Close()
			client := newTestClient(t, ts)

			mux.HandleFunc("/v1/warehouses/WH-001/clusters/CL-001/"+action, func(w http.ResponseWriter, r *http.Request) {
				if !requireMethod(t, w, r, http.MethodPost) {
					return
				}
				jsonResponse(w, 200, APIResponse[struct{}]{
					Success:   true,
					RequestID: "req-026",
				})
			})

			err := client.OperateCluster(context.Background(), "WH-001", "CL-001", action)
			if err != nil {
				t.Fatalf("OperateCluster(%s): %v", action, err)
			}
		})
	}
}

func TestConflictError(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001/clusters", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 409, map[string]any{
			"code":      "IdempotencyConflict",
			"message":   "The request conflicts with an existing idempotent request",
			"success":   false,
			"requestId": "req-conflict",
		})
	})

	_, err := client.CreateCluster(context.Background(), "WH-001", &CreateClusterRequest{
		Name:        "test",
		ClusterType: "COMPUTE",
		ComputeVcpu: 4,
		CacheGb:     100,
	})
	if err == nil {
		t.Fatal("expected error for conflict")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !apiErr.IsConflict() {
		t.Error("expected IsConflict=true")
	}
}

func TestRateLimitError(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001/clusters/CL-001", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 429, map[string]any{
			"code":      "RateLimitExceeded",
			"message":   "Request rate limit exceeded",
			"success":   false,
			"requestId": "req-rate",
		})
	})

	_, err := client.GetCluster(context.Background(), "WH-001", "CL-001")
	if err == nil {
		t.Fatal("expected error for rate limit")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !apiErr.IsRateLimited() {
		t.Error("expected IsRateLimited=true")
	}
}
