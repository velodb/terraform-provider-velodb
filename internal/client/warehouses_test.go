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
		if req.InitialCluster.Ratio == nil || *req.InitialCluster.Ratio != 8 {
			t.Errorf("expected ratio 8, got %v", req.InitialCluster.Ratio)
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
	ratio := 8
	result, err := client.CreateWarehouse(context.Background(), &CreateWarehouseRequest{
		Name:           "test-warehouse",
		DeploymentMode: "SaaS",
		CloudProvider:  "aliyun",
		Region:         "cn-beijing",
		InitialCluster: &InitialClusterRequest{
			Zone:        zone,
			ComputeVcpu: 4,
			Ratio:       &ratio,
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

func TestCreateWarehouseWithVersionAndAccessPolicy(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodPost) {
			return
		}

		var req CreateWarehouseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decoding request: %v", err)
		}
		if req.Version == nil || *req.Version != "3.0" {
			t.Errorf("expected version '3.0', got %v", req.Version)
		}
		if req.AccessPolicy == nil {
			t.Fatal("expected accessPolicy to be set")
		}
		if req.AccessPolicy.PublicAccessPolicy != "ALLOWLIST_ONLY" {
			t.Errorf("expected publicAccessPolicy 'ALLOWLIST_ONLY', got %q", req.AccessPolicy.PublicAccessPolicy)
		}
		if len(req.AccessPolicy.Rules) != 1 || req.AccessPolicy.Rules[0].CIDR != "203.0.113.0/24" {
			t.Errorf("expected one allowlist rule for 203.0.113.0/24, got %+v", req.AccessPolicy.Rules)
		}

		jsonResponse(w, 200, APIResponse[CreateWarehouseResult]{
			Success:   true,
			RequestID: "req-003",
			Data:      CreateWarehouseResult{WarehouseID: "WH-VER-001"},
		})
	})

	version := "3.0"
	pw := "asdAAQQ123"
	setupMode := "advanced"
	credentialID := int64(123)
	networkConfigID := int64(456)
	result, err := client.CreateWarehouse(context.Background(), &CreateWarehouseRequest{
		Name:            "My_Warehouse",
		DeploymentMode:  "BYOC",
		CloudProvider:   "aws",
		Region:          "us-east-1",
		Version:         &version,
		SetupMode:       &setupMode,
		CredentialID:    &credentialID,
		NetworkConfigID: &networkConfigID,
		AdminPassword:   &pw,
		AccessPolicy: &WarehousePublicAccessPolicyRequest{
			PublicAccessPolicy: "ALLOWLIST_ONLY",
			Rules:              []WarehouseAllowlistRule{{CIDR: "203.0.113.0/24", Description: "office"}},
		},
		InitialCluster: &InitialClusterRequest{Zone: "us-east-1a", ComputeVcpu: 8, CacheGb: 400},
	})
	if err != nil {
		t.Fatalf("CreateWarehouse: %v", err)
	}
	if result.WarehouseID != "WH-VER-001" {
		t.Errorf("expected 'WH-VER-001', got %q", result.WarehouseID)
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

		var raw map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("decoding request: %v", err)
		}
		body, err := json.Marshal(raw)
		if err != nil {
			t.Fatalf("encoding request for typed assertion: %v", err)
		}
		var req CreateWarehouseRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decoding typed request: %v", err)
		}

		if req.DeploymentMode != "BYOC" {
			t.Errorf("expected BYOC, got %q", req.DeploymentMode)
		}
		if req.CloudProvider != "aws" {
			t.Errorf("expected aws, got %q", req.CloudProvider)
		}
		if req.SetupMode == nil || *req.SetupMode != "advanced" {
			t.Error("expected setupMode 'advanced'")
		}
		if req.CredentialID == nil || *req.CredentialID != 123 {
			t.Error("expected credentialId 123")
		}
		if req.NetworkConfigID == nil || *req.NetworkConfigID != 456 {
			t.Error("expected networkConfigId 456")
		}
		if req.InitialCluster == nil || req.InitialCluster.Zone != "us-east-1a" {
			t.Error("expected initialCluster.zone 'us-east-1a'")
		}
		for _, field := range []string{"vpcMode", "vpcId", "bucketName", "dataCredentialArn", "deploymentCredentialArn", "subnetId", "securityGroupId", "endpointId"} {
			if _, exists := raw[field]; exists {
				t.Errorf("advanced BYOC request must not contain legacy field %q", field)
			}
		}

		jsonResponse(w, 200, APIResponse[CreateWarehouseResult]{
			Success:   true,
			RequestID: "req-002",
			Data: CreateWarehouseResult{
				WarehouseID: "WH-BYOC-001",
			},
		})
	})

	setupMode := "advanced"
	pw := "asdAAQQ123"
	zone := "us-east-1a"
	credentialID := int64(123)
	networkConfigID := int64(456)
	result, err := client.CreateWarehouse(context.Background(), &CreateWarehouseRequest{
		Name:            "My_Warehouse",
		DeploymentMode:  "BYOC",
		CloudProvider:   "aws",
		Region:          "us-east-1",
		SetupMode:       &setupMode,
		CredentialID:    &credentialID,
		NetworkConfigID: &networkConfigID,
		AdminPassword:   &pw,
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
}

func TestCreateWarehouseAcceptedIsNotComplete(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusAccepted, map[string]any{
			"success":   true,
			"requestId": "req-response",
			"data": map[string]any{
				"status":    "processing",
				"requestId": "req-original",
				"taskId":    "task-001",
			},
		})
	})

	result, err := client.CreateWarehouse(context.Background(), &CreateWarehouseRequest{
		Name:           "accepted-warehouse",
		DeploymentMode: "BYOC",
		CloudProvider:  "aws",
		Region:         "us-east-1",
	})
	if err == nil {
		t.Fatal("expected an incomplete-operation error")
	}
	if result != nil {
		t.Fatalf("expected no create result, got %#v", result)
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusAccepted || apiErr.RequestID != "req-original" {
		t.Fatalf("unexpected accepted error: %#v", apiErr)
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

func TestGetWarehouseEndpointServiceFromNestedInfo(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-NESTED", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodGet) {
			return
		}
		jsonResponse(w, 200, map[string]any{
			"success":   true,
			"requestId": "req-nested-service",
			"data": map[string]any{
				"warehouseId":   "WH-NESTED",
				"name":          "nested-service-warehouse",
				"status":        "Running",
				"cloudProvider": "aws",
				"region":        "us-east-1",
				"endpointService": map[string]any{
					"serviceId":   "vpce-svc-nested",
					"serviceName": "com.amazonaws.vpce.us-east-1.vpce-svc-nested",
				},
			},
		})
	})

	wh, err := client.GetWarehouse(context.Background(), "WH-NESTED")
	if err != nil {
		t.Fatalf("GetWarehouse: %v", err)
	}
	if wh.EndpointServiceID != "vpce-svc-nested" {
		t.Errorf("expected nested endpoint service ID, got %q", wh.EndpointServiceID)
	}
	if wh.EndpointServiceName != "com.amazonaws.vpce.us-east-1.vpce-svc-nested" {
		t.Errorf("expected nested endpoint service name, got %q", wh.EndpointServiceName)
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

func TestGetWarehouseFallsBackToListWhenDetailNotFound(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-BYOC-001", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 404, map[string]any{
			"code":      "WarehouseNotFound",
			"message":   "The warehouse [WH-BYOC-001] not found",
			"success":   false,
			"requestId": "req-detail-not-found",
		})
	})

	mux.HandleFunc("/v1/warehouses", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodGet) {
			return
		}
		if got := r.URL.Query().Get("warehouseId"); got != "WH-BYOC-001" {
			t.Fatalf("expected warehouseId filter, got %q", got)
		}
		wh := mockWarehouse("WH-BYOC-001", "byoc-warehouse")
		wh.DeploymentMode = "BYOC"
		wh.CloudProvider = "aws"
		wh.Region = "us-east-1"
		jsonResponse(w, 200, PageResponse[WarehouseItem]{
			Success:   true,
			RequestID: "req-list-fallback",
			Data:      []WarehouseItem{wh},
			Page:      1,
			Size:      100,
			Total:     1,
		})
	})

	wh, err := client.GetWarehouse(context.Background(), "WH-BYOC-001")
	if err != nil {
		t.Fatalf("GetWarehouse fallback: %v", err)
	}
	if wh.WarehouseID != "WH-BYOC-001" {
		t.Fatalf("expected fallback warehouse ID, got %q", wh.WarehouseID)
	}
	if wh.DeploymentMode != "BYOC" {
		t.Fatalf("expected BYOC deployment mode, got %q", wh.DeploymentMode)
	}
}

func TestGetWarehouseFallsBackToListWhenDetailIsEmpty(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses/WH-DELETING", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, APIResponse[WarehouseItem]{Success: true})
	})
	mux.HandleFunc("/v1/warehouses", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("warehouseId"); got != "WH-DELETING" {
			t.Fatalf("warehouseId filter = %q", got)
		}
		wh := mockWarehouse("WH-DELETING", "deleting-warehouse")
		wh.Status = "Deleting"
		jsonResponse(w, http.StatusOK, PageResponse[WarehouseItem]{
			Success: true,
			Data:    []WarehouseItem{wh},
			Total:   1,
		})
	})

	wh, err := client.GetWarehouse(context.Background(), "WH-DELETING")
	if err != nil {
		t.Fatalf("GetWarehouse: %v", err)
	}
	if wh.Status != "Deleting" {
		t.Fatalf("status = %q, want Deleting", wh.Status)
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
