package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestServer creates an httptest.Server with a mux that handles all Formation API endpoints
// using realistic mock data derived from the OpenAPI spec.
func newTestServer(t *testing.T) (*httptest.Server, *http.ServeMux) {
	t.Helper()
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)
	return ts, mux
}

// newTestClient creates a FormationClient pointed at the test server.
func newTestClient(t *testing.T, ts *httptest.Server) *FormationClient {
	t.Helper()
	host := strings.TrimPrefix(ts.URL, "http://")
	return NewFormationClient(host, "test-api-key", 0, 10*time.Second)
}

// jsonResponse writes a JSON response with the given status code.
func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// requireMethod checks the HTTP method and returns 405 if wrong.
func requireMethod(t *testing.T, w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		t.Helper()
		w.WriteHeader(http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// requireAPIKey checks the X-API-Key header.
func requireAPIKey(t *testing.T, w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("X-API-Key") == "" {
		jsonResponse(w, 401, map[string]any{
			"code":    "Unauthorized.InvalidApiKey",
			"message": "API Key not found or invalid",
			"success": false,
		})
		return false
	}
	return true
}

// --- Mock data based on OpenAPI spec ---

var mockTime = time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)

func mockWarehouse(id, name string) WarehouseItem {
	t := mockTime
	return WarehouseItem{
		WarehouseID:    id,
		Name:           name,
		Status:         "Running",
		CloudProvider:  "aliyun",
		Region:         "cn-beijing",
		Zone:           "cn-beijing-k",
		DeploymentMode: "SAAS",
		CoreVersion:    "3.0.3",
		PayType:        "PostPaid",
		CreatedAt:      &t,
		Tags:           map[string]string{"env": "test"},
	}
}

func mockCluster(id, warehouseID, name string) ClusterItem {
	t := mockTime
	return ClusterItem{
		ClusterID:     id,
		WarehouseID:   warehouseID,
		Name:          name,
		Status:        "Running",
		ClusterType:   "COMPUTE",
		CloudProvider: "aliyun",
		Region:        "cn-beijing",
		Zone:          "cn-beijing-k",
		DiskSumSize:   100,
		BillingModel:  "on_demand",
		CreatedAt:     &t,
		StartedAt:     &t,
		ConnectionInfo: &ClusterConnectionInfo{
			PublicEndpoint:  fmt.Sprintf("%s.selectdbcloud.com", id),
			PrivateEndpoint: fmt.Sprintf("%s.internal", id),
			ListenerPort:    9030,
		},
	}
}
