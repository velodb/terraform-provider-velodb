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
		if req.DeploymentMode != "SAAS" {
			t.Errorf("expected deploymentMode 'SAAS', got %q", req.DeploymentMode)
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
		DeploymentMode: "SAAS",
		CloudProvider:  "aliyun",
		Region:         "cn-beijing",
		InitialCluster: &InitialClusterRequest{
			Name:        "default",
			Zone:        &zone,
			ComputeVcpu: 4,
			CacheGb:     1000,
			AutoPause:   &AutoPauseConfig{Enabled: false},
		},
		AdvancedSettings: map[string]any{"enableTde": 0},
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
		if req.VpcID == nil || *req.VpcID != "vpc-xxxxxx" {
			t.Error("expected vpcId 'vpc-xxxxxx'")
		}
		if req.SetupMode == nil || *req.SetupMode != "guided" {
			t.Error("expected setupMode 'guided'")
		}

		jsonResponse(w, 200, APIResponse[CreateWarehouseResult]{
			Success:   true,
			RequestID: "req-002",
			Data: CreateWarehouseResult{
				WarehouseID: "WH-BYOC-001",
				ByocSetup: &WarehouseByocSetup{
					Token:        "tok-abc123",
					ShellCommand: "curl https://setup.example.com | bash",
					URL:          "https://setup.example.com/template",
					DocURL:       "https://docs.example.com/byoc",
				},
			},
		})
	})

	vpcMode := "existing"
	vpcID := "vpc-xxxxxx"
	setupMode := "guided"
	pw := "asdAAQQ123"
	result, err := client.CreateWarehouse(context.Background(), &CreateWarehouseRequest{
		Name:           "My_Warehouse",
		DeploymentMode: "BYOC",
		CloudProvider:  "aliyun",
		Region:         "cn-beijing",
		VpcMode:        &vpcMode,
		VpcID:          &vpcID,
		SetupMode:     &setupMode,
		AdminPassword:  &pw,
		InitialCluster: &InitialClusterRequest{
			Name:        "default-compute",
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
	if result.ByocSetup == nil {
		t.Fatal("expected byocSetup to be returned")
	}
	if result.ByocSetup.ShellCommand == "" {
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

func TestUpdateWarehouseSettings(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001/settings", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodPatch) {
			return
		}
		var req UpdateWarehouseSettingsRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.MaintenanceWindow == nil {
			t.Fatal("expected maintenanceWindow")
		}
		if req.MaintenanceWindow.StartHourUtc != 2 || req.MaintenanceWindow.EndHourUtc != 4 {
			t.Errorf("expected start=2 end=4, got %+v", req.MaintenanceWindow)
		}
		if req.UpgradePolicy == nil || *req.UpgradePolicy != "automatic" {
			t.Errorf("expected upgradePolicy 'automatic'")
		}

		jsonResponse(w, 200, APIResponse[struct{}]{
			Success:   true,
			RequestID: "req-008",
		})
	})

	policy := "automatic"
	err := client.UpdateWarehouseSettings(context.Background(), "WH-001", &UpdateWarehouseSettingsRequest{
		UpgradePolicy:     &policy,
		MaintenanceWindow: &MaintenanceWindow{StartHourUtc: 2, EndHourUtc: 4},
	})
	if err != nil {
		t.Fatalf("UpdateWarehouseSettings: %v", err)
	}
}

func TestGetWarehouseSettings(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-001/settings", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodGet) {
			return
		}
		jsonResponse(w, 200, APIResponse[WarehouseSettingsResponse]{
			Success:   true,
			RequestID: "req-009",
			Data: WarehouseSettingsResponse{
				UpgradePolicy:     "automatic",
				MaintenanceWindow: &MaintenanceWindow{StartHourUtc: 2, EndHourUtc: 4},
			},
		})
	})

	settings, err := client.GetWarehouseSettings(context.Background(), "WH-001")
	if err != nil {
		t.Fatalf("GetWarehouseSettings: %v", err)
	}
	if settings.UpgradePolicy != "automatic" {
		t.Errorf("expected upgradePolicy 'automatic', got %q", settings.UpgradePolicy)
	}
	if settings.MaintenanceWindow == nil || settings.MaintenanceWindow.StartHourUtc != 2 {
		t.Errorf("expected startHourUtc=2, got %+v", settings.MaintenanceWindow)
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

func TestGetWarehouseByocSetup(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-BYOC/byoc-setup", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodGet) {
			return
		}
		jsonResponse(w, 200, APIResponse[WarehouseByocSetup]{
			Success:   true,
			RequestID: "req-012",
			Data: WarehouseByocSetup{
				Token:                 "tok-abc",
				ShellCommand:          "curl https://setup.example.com | bash",
				ShellCommandForNewVpc: "curl https://setup.example.com/new-vpc | bash",
				URL:                   "https://setup.example.com/template",
				DocURL:                "https://docs.example.com/byoc",
			},
		})
	})

	setup, err := client.GetWarehouseByocSetup(context.Background(), "WH-BYOC")
	if err != nil {
		t.Fatalf("GetWarehouseByocSetup: %v", err)
	}
	if setup.Token != "tok-abc" {
		t.Errorf("expected token 'tok-abc', got %q", setup.Token)
	}
	if setup.ShellCommand == "" {
		t.Error("expected shellCommand")
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
		jp := 9030
		hp := 8030
		slp := 8040
		jsonResponse(w, 200, APIResponse[WarehouseConnections]{
			Success:   true,
			RequestID: "req-013",
			Data: WarehouseConnections{
				PublicConnection: &WarehousePublicConnection{
					Host:           "wh-001.selectdbcloud.com",
					JdbcPort:       &jp,
					HTTPPort:       &hp,
					StreamLoadPort: &slp,
				},
			},
		})
	})

	conns, err := client.GetWarehouseConnections(context.Background(), "WH-001")
	if err != nil {
		t.Fatalf("GetWarehouseConnections: %v", err)
	}
	if conns.PublicConnection == nil {
		t.Fatal("expected publicConnection")
	}
	if conns.PublicConnection.Host != "wh-001.selectdbcloud.com" {
		t.Errorf("expected host 'wh-001.selectdbcloud.com', got %q", conns.PublicConnection.Host)
	}
	if conns.PublicConnection.JdbcPort == nil || *conns.PublicConnection.JdbcPort != 9030 {
		t.Errorf("expected jdbcPort 9030")
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
