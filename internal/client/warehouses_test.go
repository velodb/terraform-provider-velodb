package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestCreateWarehouse(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodPost) {
			return
		}
		if !requireAPIKey(t, w, r) {
			return
		}

		// Verify RequestId header is present for write operations
		if r.Header.Get("RequestId") == "" {
			t.Error("expected RequestId header for POST")
		}

		var req CreateWarehouseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decoding request: %v", err)
		}
		if req.Name != "test-warehouse" {
			t.Errorf("expected name 'test-warehouse', got %q", req.Name)
		}
		if req.DeploymentMode != "SaaS" {
			t.Errorf("expected deploymentMode 'SaaS', got %q", req.DeploymentMode)
		}
		if req.CloudProvider != "aliyun" {
			t.Errorf("expected cloudProvider 'aliyun', got %q", req.CloudProvider)
		}
		if req.Region != "cn-beijing" {
			t.Errorf("expected region 'cn-beijing', got %q", req.Region)
		}
		if req.InitialCluster == nil {
			t.Fatal("expected initialCluster to be set")
		}
		if req.InitialCluster.ComputeVcpu != 4 {
			t.Errorf("expected computeVcpu 4, got %d", req.InitialCluster.ComputeVcpu)
		}

		jsonResponse(w, 200, APIResponse[CreateWarehouseResult]{
			Success:   true,
			RequestID: "req-001",
			Data: CreateWarehouseResult{
				WarehouseID: "WH-TEST-001",
			},
		})
	})

	zone := "cn-beijing-k"
	result, err := client.CreateWarehouse(context.Background(), &CreateWarehouseRequest{
		Name:           "test-warehouse",
		DeploymentMode: "SaaS",
		CloudProvider:  "aliyun",
		Region:         "cn-beijing",
		InitialCluster: &InitialClusterRequest{
			Zone:        zone,
			ComputeVcpu: 4,
			CacheGb:     1000,
			AutoPause:   &AutoPauseConfig{Enabled: false},
		},
	})
	if err != nil {
		t.Fatalf("CreateWarehouse: %v", err)
	}
	if result.WarehouseID != "WH-TEST-001" {
		t.Errorf("expected warehouseId 'WH-TEST-001', got %q", result.WarehouseID)
	}
}

func TestCreateWarehouseBYOC(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodPost) {
			return
		}

		var req CreateWarehouseRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.DeploymentMode != "BYOC" {
			t.Errorf("expected BYOC, got %q", req.DeploymentMode)
		}
		if req.VpcMode == nil || *req.VpcMode != "existing" {
			t.Error("expected vpcMode 'existing'")
		}
		if req.SetupMode == nil || *req.SetupMode != "guided" {
			t.Error("expected setupMode 'guided'")
		}
		if req.InitialCluster == nil || req.InitialCluster.Zone != "cn-beijing-k" {
			t.Error("expected initialCluster.zone 'cn-beijing-k'")
		}

		jsonResponse(w, 200, APIResponse[CreateWarehouseResult]{
			Success:   true,
			RequestID: "req-002",
			Data: CreateWarehouseResult{
				WarehouseID: "WH-BYOC-001",
				SetupGuide: &WarehouseSetupGuide{
					ShellCommand: "curl https://setup.example.com | bash",
					SetupURL:     "https://setup.example.com/template",
					GuideURL:     "https://docs.example.com/byoc",
				},
			},
		})
	})

	vpcMode := "existing"
	setupMode := "guided"
	pw := "asdAAQQ123"
	zone := "cn-beijing-k"
	result, err := client.CreateWarehouse(context.Background(), &CreateWarehouseRequest{
		Name:           "My_Warehouse",
		DeploymentMode: "BYOC",
		CloudProvider:  "aliyun",
		Region:         "cn-beijing",
		VpcMode:        &vpcMode,
		SetupMode:      &setupMode,
		AdminPassword:  &pw,
		InitialCluster: &InitialClusterRequest{
			Zone:        zone,
			ComputeVcpu: 8,
			CacheGb:     400,
		},
	})
	if err != nil {
		t.Fatalf("CreateWarehouse BYOC: %v", err)
	}
	if result.WarehouseID != "WH-BYOC-001" {
		t.Errorf("expected 'WH-BYOC-001', got %q", result.WarehouseID)
	}
	if result.SetupGuide == nil {
		t.Fatal("expected setupGuide to be returned")
	}
	if result.SetupGuide.ShellCommand == "" {
		t.Error("expected shellCommand in BYOC setup")
	}
}

func TestGetWarehouse(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodGet) {
			return
		}
		if !requireAPIKey(t, w, r) {
			return
		}
		jsonResponse(w, 200, APIResponse[WarehouseItem]{
			Success:   true,
			RequestID: "req-003",
			Data:      mockWarehouse("WH-001", "test-warehouse"),
		})
	})

	wh, err := client.GetWarehouse(context.Background(), "WH-001")
	if err != nil {
		t.Fatalf("GetWarehouse: %v", err)
	}
	if wh.WarehouseID != "WH-001" {
		t.Errorf("expected 'WH-001', got %q", wh.WarehouseID)
	}
	if wh.Name != "test-warehouse" {
		t.Errorf("expected 'test-warehouse', got %q", wh.Name)
	}
	if wh.Status != "Running" {
		t.Errorf("expected 'Running', got %q", wh.Status)
	}
	if wh.CloudProvider != "aliyun" {
		t.Errorf("expected 'aliyun', got %q", wh.CloudProvider)
	}
}

func TestGetWarehouseNotFound(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-MISSING", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 404, map[string]any{
			"code":      "WarehouseNotFound",
			"message":   "The warehouse [WH-MISSING] not found",
			"success":   false,
			"requestId": "req-004",
		})
	})

	_, err := client.GetWarehouse(context.Background(), "WH-MISSING")
	if err == nil {
		t.Fatal("expected error for missing warehouse")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !apiErr.IsNotFound() {
		t.Errorf("expected IsNotFound=true, got false (code=%q)", apiErr.Code)
	}
}

func TestListWarehouses(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodGet) {
			return
		}

		// Verify query params
		q := r.URL.Query()
		if q.Get("page") != "1" {
			t.Errorf("expected page=1, got %q", q.Get("page"))
		}
		if q.Get("size") != "20" {
			t.Errorf("expected size=20, got %q", q.Get("size"))
		}
		if q.Get("cloudProvider") != "aliyun" {
			t.Errorf("expected cloudProvider=aliyun, got %q", q.Get("cloudProvider"))
		}

		jsonResponse(w, 200, PageResponse[WarehouseItem]{
			Success:   true,
			RequestID: "req-005",
			Data: []WarehouseItem{
				mockWarehouse("WH-001", "warehouse-a"),
				mockWarehouse("WH-002", "warehouse-b"),
			},
			Page:  1,
			Size:  20,
			Total: 2,
		})
	})

	result, err := client.ListWarehouses(context.Background(), &ListWarehousesOptions{
		Page:          1,
		Size:          20,
		CloudProvider: "aliyun",
	})
	if err != nil {
		t.Fatalf("ListWarehouses: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected total=2, got %d", result.Total)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 warehouses, got %d", len(result.Data))
	}
	if result.Data[0].Name != "warehouse-a" {
		t.Errorf("expected 'warehouse-a', got %q", result.Data[0].Name)
	}
}

func TestUpdateWarehouse(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodPatch) {
			return
		}
		var req UpdateWarehouseRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Name == nil || *req.Name != "renamed-warehouse" {
			t.Errorf("expected name 'renamed-warehouse'")
		}

		jsonResponse(w, 200, APIResponse[struct{}]{
			Success:   true,
			RequestID: "req-006",
		})
	})

	name := "renamed-warehouse"
	err := client.UpdateWarehouse(context.Background(), "WH-001", &UpdateWarehouseRequest{
		Name: &name,
	})
	if err != nil {
		t.Fatalf("UpdateWarehouse: %v", err)
	}
}

func TestDeleteWarehouse(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodDelete) {
			return
		}
		if r.Header.Get("RequestId") == "" {
			t.Error("expected RequestId header for DELETE")
		}
		jsonResponse(w, 200, APIResponse[struct{}]{
			Success:   true,
			RequestID: "req-007",
		})
	})

	err := client.DeleteWarehouse(context.Background(), "WH-001")
	if err != nil {
		t.Fatalf("DeleteWarehouse: %v", err)
	}
}

func TestUpgradeWarehouse(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001/settings/upgrade", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodPost) {
			return
		}
		var req UpgradeWarehouseRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.TargetVersionID != 42 {
			t.Errorf("expected targetVersionId 42, got %d", req.TargetVersionID)
		}
		jsonResponse(w, 200, APIResponse[struct{}]{
			Success:   true,
			RequestID: "req-010",
		})
	})

	err := client.UpgradeWarehouse(context.Background(), "WH-001", 42)
	if err != nil {
		t.Fatalf("UpgradeWarehouse: %v", err)
	}
}

func TestChangeWarehousePassword(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001/settings/password", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodPost) {
			return
		}
		var req ChangePasswordRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.NewPassword != "NewPass@12345.aA" {
			t.Errorf("expected newPassword 'NewPass@12345.aA', got %q", req.NewPassword)
		}
		jsonResponse(w, 200, APIResponse[struct{}]{
			Success:   true,
			RequestID: "req-011",
		})
	})

	err := client.ChangeWarehousePassword(context.Background(), "WH-001", "NewPass@12345.aA")
	if err != nil {
		t.Fatalf("ChangeWarehousePassword: %v", err)
	}
}

func TestGetWarehouseConnections(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001/connections", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodGet) {
			return
		}
		jsonResponse(w, 200, APIResponse[WarehouseConnections]{
			Success:   true,
			RequestID: "req-013",
			Data: WarehouseConnections{
				PublicEndpoints: []ConnectionEndpoint{
					{Protocol: "jdbc", Host: "wh-001.selectdbcloud.com", Port: 9030},
					{Protocol: "http", Host: "wh-001.selectdbcloud.com", Port: 8030},
				},
				PrivateEndpoints: []PrivateConnectionEndpoint{
					{ConnectionEndpoint: ConnectionEndpoint{Protocol: "jdbc", Host: "wh-001.internal", Port: 9030}, EndpointID: "vpce-001"},
				},
				ComputeClusters: []ConnectionCluster{
					{ClusterID: "CL-001", ClusterName: "default", HTTPPort: 9050},
				},
			},
		})
	})

	conns, err := client.GetWarehouseConnections(context.Background(), "WH-001")
	if err != nil {
		t.Fatalf("GetWarehouseConnections: %v", err)
	}
	if len(conns.PublicEndpoints) != 2 {
		t.Fatalf("expected 2 public endpoints, got %d", len(conns.PublicEndpoints))
	}
	if conns.PublicEndpoints[0].Host != "wh-001.selectdbcloud.com" {
		t.Errorf("expected host 'wh-001.selectdbcloud.com', got %q", conns.PublicEndpoints[0].Host)
	}
	if conns.PublicEndpoints[0].Port != 9030 {
		t.Errorf("expected port 9030, got %d", conns.PublicEndpoints[0].Port)
	}
	if len(conns.PrivateEndpoints) != 1 {
		t.Fatalf("expected 1 private endpoint, got %d", len(conns.PrivateEndpoints))
	}
	if conns.PrivateEndpoints[0].EndpointID != "vpce-001" {
		t.Errorf("expected endpoint ID 'vpce-001', got %q", conns.PrivateEndpoints[0].EndpointID)
	}
}

func TestUnauthorizedError(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()

	// Create a client with empty API key — transport still sets it, so mock the server to reject
	host := ts.URL[len("http://"):]
	c := NewFormationClient(host, "", 0, 10*time.Second)

	mux.HandleFunc("/v1/warehouses", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 401, map[string]any{
			"code":      "Unauthorized.InvalidApiKey",
			"message":   "API Key not found or invalid",
			"success":   false,
			"requestId": "req-err-001",
		})
	})

	_, err := c.ListWarehouses(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("expected 401, got %d", apiErr.StatusCode)
	}
	if apiErr.Code != "Unauthorized.InvalidApiKey" {
		t.Errorf("expected code 'Unauthorized.InvalidApiKey', got %q", apiErr.Code)
	}
}
