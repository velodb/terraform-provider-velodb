package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// mockAPIServer creates a mock Formation API server with all endpoints needed for acceptance tests.
func mockAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	whDeleted := false
	clDeleted := false

	// -- Warehouse endpoints --
	mux.HandleFunc("/v1/warehouses", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			data := []map[string]any{}
			if !whDeleted {
				data = append(data, map[string]any{
					"warehouseId": "WH-MOCK-001", "name": "mock-warehouse", "status": "Running",
					"cloudProvider": "aliyun", "region": "cn-beijing", "zone": "cn-beijing-k",
					"deploymentMode": "SAAS", "coreVersion": "3.0.3", "payType": "PostPaid",
					"createdAt": now.Format(time.RFC3339),
				})
			}
			json.NewEncoder(w).Encode(map[string]any{
				"success": true, "requestId": "mock-list-wh", "page": 1, "size": 20,
				"total": len(data), "data": data,
			})
		case http.MethodPost:
			whDeleted = false
			json.NewEncoder(w).Encode(map[string]any{
				"success": true, "requestId": "mock-create-wh",
				"data": map[string]any{"warehouseId": "WH-MOCK-001"},
			})
		default:
			w.WriteHeader(405)
		}
	})

	mux.HandleFunc("/v1/warehouses/WH-MOCK-001", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if whDeleted && r.Method == http.MethodGet {
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(map[string]any{
				"code": "WarehouseNotFound", "message": "not found", "success": false, "requestId": "mock",
			})
			return
		}
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{
				"success": true, "requestId": "mock-get-wh",
				"data": map[string]any{
					"warehouseId": "WH-MOCK-001", "name": "mock-warehouse", "status": "Running",
					"cloudProvider": "aliyun", "region": "cn-beijing", "zone": "cn-beijing-k",
					"deploymentMode": "SAAS", "coreVersion": "3.0.3", "payType": "PostPaid",
					"createdAt": now.Format(time.RFC3339),
				},
			})
		case http.MethodPatch:
			json.NewEncoder(w).Encode(map[string]any{"success": true, "requestId": "mock-update-wh", "data": map[string]any{}})
		case http.MethodDelete:
			whDeleted = true
			json.NewEncoder(w).Encode(map[string]any{"success": true, "requestId": "mock-delete-wh", "data": map[string]any{}})
		}
	})

	mux.HandleFunc("/v1/warehouses/WH-MOCK-001/settings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"success": true, "requestId": "mock-settings", "data": map[string]any{}})
	})

	mux.HandleFunc("/v1/warehouses/WH-MOCK-001/byoc-setup", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]any{"code": "NotFound", "message": "Not BYOC", "success": false})
	})

	mux.HandleFunc("/v1/warehouses/WH-MOCK-001/connections", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success":   true,
			"requestId": "mock-conns",
			"data": map[string]any{
				"warehouseId": "WH-MOCK-001",
				"clusters": []map[string]any{
					{
						"clusterId":       "CL-MOCK-001",
						"type":            "SQL",
						"jdbcPort":        9030,
						"httpPort":        8030,
						"streamLoadPort":  8040,
						"publicEndpoint":  "mock.selectdbcloud.com",
						"privateEndpoint": "mock.internal",
						"listenerPort":    9030,
					},
				},
			},
		})
	})

	// -- Cluster endpoints --
	clusterData := func() map[string]any {
		return map[string]any{
			"clusterId": "CL-MOCK-001", "warehouseId": "WH-MOCK-001",
			"name": "mock-cluster", "status": "Running", "clusterType": "COMPUTE",
			"cloudProvider": "aliyun", "region": "cn-beijing", "zone": "cn-beijing-k",
			"payType": "PostPaid", "createdAt": now.Format(time.RFC3339),
			"connectionInfo": map[string]any{
				"publicEndpoint": "cl-mock.selectdbcloud.com", "privateEndpoint": "cl-mock.internal", "listenerPort": 9030,
			},
		}
	}

	mux.HandleFunc("/v1/warehouses/WH-MOCK-001/clusters", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			data := []map[string]any{}
			if !clDeleted {
				data = append(data, clusterData())
			}
			json.NewEncoder(w).Encode(map[string]any{
				"success": true, "requestId": "mock-list-cl", "page": 1, "size": 20,
				"total": len(data), "data": data,
			})
		case http.MethodPost:
			clDeleted = false
			json.NewEncoder(w).Encode(map[string]any{
				"success": true, "requestId": "mock-create-cl",
				"data": map[string]any{"clusterId": "CL-MOCK-001"},
			})
		}
	})

	mux.HandleFunc("/v1/warehouses/WH-MOCK-001/clusters/CL-MOCK-001", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if clDeleted && r.Method == http.MethodGet {
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(map[string]any{
				"code": "ClusterNotFound", "message": "not found", "success": false, "requestId": "mock",
			})
			return
		}
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{
				"success": true, "requestId": "mock-get-cl", "data": clusterData(),
			})
		case http.MethodPatch:
			json.NewEncoder(w).Encode(map[string]any{"success": true, "requestId": "mock-update-cl", "data": map[string]any{}})
		case http.MethodDelete:
			clDeleted = true
			json.NewEncoder(w).Encode(map[string]any{"success": true, "requestId": "mock-delete-cl", "data": map[string]any{}})
		}
	})

	for _, action := range []string{"pause", "resume", "reboot"} {
		mux.HandleFunc("/v1/warehouses/WH-MOCK-001/clusters/CL-MOCK-001/"+action, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": true, "requestId": "mock-action-cl", "data": map[string]any{}})
		})
	}

	return httptest.NewServer(mux)
}

func testAccProtoV6ProviderFactories(ts *httptest.Server) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"velodb": providerserver.NewProtocol6WithError(New("test")()),
	}
}

func testProviderConfig(ts *httptest.Server) string {
	host := strings.TrimPrefix(ts.URL, "http://")
	return fmt.Sprintf(`
provider "velodb" {
  host    = %q
  api_key = "test-api-key"
}
`, host)
}

func TestAccWarehouseResource(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
resource "velodb_warehouse" "test" {
  name            = "mock-warehouse"
  deployment_mode = "SAAS"
  cloud_provider  = "aliyun"
  region          = "cn-beijing"

  admin_password         = "TestPass@123"
  admin_password_version = 1

  initial_cluster {
    name         = "default"
    zone         = "cn-beijing-k"
    compute_vcpu = 4
    cache_gb     = 1000
    auto_pause {
      enabled              = false
      idle_timeout_minutes = 30
    }
  }

  timeouts {
    create = "1m"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("velodb_warehouse.test", "id", "WH-MOCK-001"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "name", "mock-warehouse"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "status", "Running"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "cloud_provider", "aliyun"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "region", "cn-beijing"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "deployment_mode", "SAAS"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "core_version", "3.0.3"),
				),
			},
			// Import
			{
				ResourceName:            "velodb_warehouse.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"admin_password", "admin_password_version", "initial_cluster", "advanced_settings", "timeouts"},
			},
		},
	})
}

func TestAccClusterResource(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
resource "velodb_cluster" "test" {
  warehouse_id   = "WH-MOCK-001"
  name           = "mock-cluster"
  cluster_type   = "COMPUTE"
  compute_vcpu   = 4
  cache_gb       = 100
  desired_state  = "running"

  auto_pause {
    enabled              = true
    idle_timeout_minutes = 15
  }

  timeouts {
    create = "1m"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("velodb_cluster.test", "id", "CL-MOCK-001"),
					resource.TestCheckResourceAttr("velodb_cluster.test", "warehouse_id", "WH-MOCK-001"),
					resource.TestCheckResourceAttr("velodb_cluster.test", "name", "mock-cluster"),
					resource.TestCheckResourceAttr("velodb_cluster.test", "status", "Running"),
					resource.TestCheckResourceAttr("velodb_cluster.test", "cluster_type", "COMPUTE"),
					resource.TestCheckResourceAttr("velodb_cluster.test", "desired_state", "running"),
					resource.TestCheckResourceAttr("velodb_cluster.test", "connection_info.0.public_endpoint", "cl-mock.selectdbcloud.com"),
					resource.TestCheckResourceAttr("velodb_cluster.test", "connection_info.0.listener_port", "9030"),
				),
			},
			// Import
			{
				ResourceName:            "velodb_cluster.test",
				ImportState:             true,
				ImportStateId:           "WH-MOCK-001/CL-MOCK-001",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"desired_state", "billing_method", "auto_pause", "timeouts", "compute_vcpu", "cache_gb"},
			},
		},
	})
}

func TestAccWarehousesDataSource(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
data "velodb_warehouses" "test" {
  cloud_provider = "aliyun"
  region         = "cn-beijing"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "total", "1"),
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "warehouses.#", "1"),
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "warehouses.0.warehouse_id", "WH-MOCK-001"),
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "warehouses.0.name", "mock-warehouse"),
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "warehouses.0.status", "Running"),
				),
			},
		},
	})
}

func TestAccClustersDataSource(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
data "velodb_clusters" "test" {
  warehouse_id = "WH-MOCK-001"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.velodb_clusters.test", "total", "1"),
					resource.TestCheckResourceAttr("data.velodb_clusters.test", "clusters.#", "1"),
					resource.TestCheckResourceAttr("data.velodb_clusters.test", "clusters.0.cluster_id", "CL-MOCK-001"),
					resource.TestCheckResourceAttr("data.velodb_clusters.test", "clusters.0.name", "mock-cluster"),
				),
			},
		},
	})
}

func TestAccWarehouseConnectionsDataSource(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
data "velodb_warehouse_connections" "test" {
  warehouse_id = "WH-MOCK-001"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "clusters.#", "1"),
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "clusters.0.cluster_id", "CL-MOCK-001"),
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "clusters.0.jdbc_port", "9030"),
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "clusters.0.http_port", "8030"),
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "clusters.0.public_endpoint", "mock.selectdbcloud.com"),
				),
			},
		},
	})
}
