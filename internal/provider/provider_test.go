package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
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
	publicPolicy := "DENY_ALL"
	publicPolicyRules := []map[string]any{}

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
					"deploymentMode": "SaaS", "coreVersion": "3.0.3", "payType": "PostPaid",
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
					"deploymentMode": "SaaS", "coreVersion": "3.0.3", "payType": "PostPaid",
					"endpointServiceId": "vpce-svc-mock", "endpointServiceName": "com.amazonaws.vpce.cn-beijing.vpce-svc-mock",
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

	mux.HandleFunc("/v1/warehouses/WH-BYOC-001", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success": true, "requestId": "mock-get-byoc-wh",
			"data": map[string]any{
				"warehouseId": "WH-BYOC-001", "name": "mock-byoc", "status": "Running",
				"cloudProvider": "aws", "region": "us-east-1", "zone": "us-east-1a",
				"deploymentMode": "BYOC", "coreVersion": "3.0.3", "payType": "PostPaid",
				"endpointServiceName": "com.amazonaws.vpce.us-east-1.vpce-svc-byoc",
				"setupGuide": map[string]any{
					"shellCommand": "curl https://setup.example.com | bash",
					"setupUrl":     "https://setup.example.com/template",
					"guideUrl":     "https://docs.example.com/byoc",
				},
				"createdAt": now.Format(time.RFC3339),
			},
		})
	})

	mux.HandleFunc("/v1/warehouses/WH-BYOC-001/clusters", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success": true, "requestId": "mock-list-byoc-cl", "page": 1, "size": 20,
			"total": 1,
			"data": []map[string]any{{
				"clusterId": "CL-BYOC-001", "warehouseId": "WH-BYOC-001",
				"name": "byoc_cluster", "status": "Running", "clusterType": "COMPUTE",
				"cloudProvider": "aws", "region": "us-east-1", "zone": "us-east-1a",
			}},
		})
	})

	mux.HandleFunc("/v1/warehouses/WH-MOCK-001/connections", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success":   true,
			"requestId": "mock-conns",
			"data": map[string]any{
				"publicEndpoints": []map[string]any{
					{"protocol": "jdbc", "host": "mock.selectdbcloud.com", "port": 9030},
					{"protocol": "http", "host": "mock.selectdbcloud.com", "port": 8030},
				},
				"privateEndpoints": []map[string]any{
					{"protocol": "jdbc", "host": "mock.internal", "port": 9030, "endpointId": "vpce-mock"},
				},
				"computeClusters": []map[string]any{
					{"clusterId": "CL-MOCK-001", "clusterName": "mock_cluster", "httpPort": 9050},
				},
			},
		})
	})

	mux.HandleFunc("/v1/warehouses/WH-MOCK-001/connections/public/access-policy", func(w http.ResponseWriter, r *http.Request) {
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
				"success": true, "requestId": "mock-get-public-policy",
				"data": map[string]any{
					"publicAccessPolicy": publicPolicy,
					"allowlist":          publicPolicyRules,
				},
			})
		case http.MethodPatch:
			var body struct {
				PublicAccessPolicy string           `json:"publicAccessPolicy"`
				Rules              []map[string]any `json:"rules"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]any{"success": false, "message": err.Error()})
				return
			}
			publicPolicy = body.PublicAccessPolicy
			publicPolicyRules = []map[string]any{}
			if publicPolicy == "ALLOWLIST_ONLY" {
				publicPolicyRules = body.Rules
			}
			json.NewEncoder(w).Encode(map[string]any{"success": true, "requestId": "mock-update-public-policy", "data": map[string]any{}})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// -- Cluster endpoints --
	clusterData := func() map[string]any {
		return map[string]any{
			"clusterId": "CL-MOCK-001", "warehouseId": "WH-MOCK-001",
			"name": "mock_cluster", "status": "Running", "clusterType": "COMPUTE",
			"cloudProvider": "aliyun", "region": "cn-beijing", "zone": "cn-beijing-k",
			"billingModel": "on_demand", "createdAt": now.Format(time.RFC3339),
			"billingPools": map[string]any{
				"onDemand": map[string]any{"nodeCount": 1, "cpu": 4, "diskSizeGb": 100},
			},
			"billingSummary": map[string]any{
				"isMixedBilling": false, "nodeCount": 1, "onDemandNodeCount": 1,
				"totalCpu": 4, "totalDiskSizeGb": 100,
			},
			"connectionInfo": map[string]any{
				"publicEndpoint": "cl-mock.selectdbcloud.com", "privateEndpoint": "cl-mock.internal", "listenerPort": 9030,
			},
			"autoPause": map[string]any{"enabled": true, "idleTimeoutMinutes": 15},
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
  deployment_mode = "SaaS"
  cloud_provider  = "aliyun"
  region          = "cn-beijing"

  admin_password         = "TestPass@123"
  admin_password_version = 1

  initial_cluster {
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
					resource.TestCheckResourceAttr("velodb_warehouse.test", "deployment_mode", "SaaS"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "core_version", "3.0.3"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "endpoint_service_name", "com.amazonaws.vpce.cn-beijing.vpce-svc-mock"),
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

func TestWarehouseImportMissingFails(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
resource "velodb_warehouse" "test" {
  name            = "mock-warehouse"
  deployment_mode = "SaaS"
  cloud_provider  = "aliyun"
  region          = "cn-beijing"

  admin_password         = "TestPass@123"
  admin_password_version = 1

  initial_cluster {
    zone         = "cn-beijing-k"
    compute_vcpu = 4
    cache_gb     = 100
  }
}
`,
			},
			{
				ResourceName:  "velodb_warehouse.test",
				ImportState:   true,
				ImportStateId: "WH-MISSING",
				ExpectError:   regexp.MustCompile("Warehouse not found"),
			},
		},
	})
}

func TestWarehouseImportBYOC(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
resource "velodb_warehouse" "byoc" {
  name            = "mock-byoc"
  deployment_mode = "BYOC"
  cloud_provider  = "aws"
  region          = "us-east-1"
}
`,
				ResourceName:            "velodb_warehouse.byoc",
				ImportState:             true,
				ImportStateId:           "WH-BYOC-001",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("velodb_warehouse.byoc", "id", "WH-BYOC-001"),
					resource.TestCheckResourceAttr("velodb_warehouse.byoc", "name", "mock-byoc"),
					resource.TestCheckResourceAttr("velodb_warehouse.byoc", "deployment_mode", "BYOC"),
					resource.TestCheckResourceAttr("velodb_warehouse.byoc", "cloud_provider", "aws"),
					resource.TestCheckResourceAttr("velodb_warehouse.byoc", "region", "us-east-1"),
					resource.TestCheckResourceAttr("velodb_warehouse.byoc", "initial_cluster_id", "CL-BYOC-001"),
					resource.TestCheckResourceAttr("velodb_warehouse.byoc", "byoc_setup.0.shell_command", "curl https://setup.example.com | bash"),
				),
			},
		},
	})
}

func TestWarehouseCreateBYOCFails(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
resource "velodb_warehouse" "byoc" {
  name            = "mock-byoc"
  deployment_mode = "BYOC"
  cloud_provider  = "aws"
  region          = "us-east-1"
  setup_mode      = "guided"
  vpc_mode        = "existing"

  admin_password = "TestPass@123"

  initial_cluster {
    zone         = "us-east-1a"
    compute_vcpu = 8
    cache_gb     = 400
  }
}
`,
				ExpectError: regexp.MustCompile("BYOC warehouse creation is not supported"),
			},
		},
	})
}

func TestWarehouseAutoPauseRequiresTimeoutWhenEnabled(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
resource "velodb_warehouse" "test" {
  name            = "mock-warehouse"
  deployment_mode = "SaaS"
  cloud_provider  = "aliyun"
  region          = "cn-beijing"

  admin_password         = "TestPass@123"
  admin_password_version = 1

  initial_cluster {
    zone         = "cn-beijing-k"
    compute_vcpu = 4
    cache_gb     = 100
    auto_pause {
      enabled = true
    }
  }
}
`,
				ExpectError: regexp.MustCompile("idle_timeout_minutes is required when auto_pause is enabled"),
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
  warehouse_id  = "WH-MOCK-001"
  name          = "mock_cluster"
  cluster_type  = "COMPUTE"
  compute_vcpu  = 4
  cache_gb      = 100
  desired_state = "running"

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
					resource.TestCheckResourceAttr("velodb_cluster.test", "name", "mock_cluster"),
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
				ImportStateVerifyIgnore: []string{"desired_state", "auto_pause", "timeouts"},
			},
		},
	})
}

func TestClusterImportMissingFails(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
resource "velodb_cluster" "test" {
  warehouse_id  = "WH-MOCK-001"
  name          = "mock_cluster"
  cluster_type  = "COMPUTE"
  compute_vcpu  = 4
  cache_gb      = 100
}
`,
			},
			{
				ResourceName:  "velodb_cluster.test",
				ImportState:   true,
				ImportStateId: "WH-MOCK-001/CL-MISSING",
				ExpectError:   regexp.MustCompile("Cluster not found"),
			},
		},
	})
}

func TestClusterAutoPauseRequiresTimeoutWhenEnabled(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
resource "velodb_cluster" "test" {
  warehouse_id  = "WH-MOCK-001"
  name          = "mock_cluster"
  cluster_type  = "COMPUTE"
  compute_vcpu  = 4
  cache_gb      = 100

  auto_pause {
    enabled = true
  }
}
`,
				ExpectError: regexp.MustCompile("idle_timeout_minutes is required when auto_pause is enabled"),
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
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "warehouses.0.endpoint_service_id", "vpce-svc-mock"),
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "warehouses.0.endpoint_service_name", "com.amazonaws.vpce.cn-beijing.vpce-svc-mock"),
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
					resource.TestCheckResourceAttr("data.velodb_clusters.test", "clusters.0.name", "mock_cluster"),
					resource.TestCheckResourceAttr("data.velodb_clusters.test", "clusters.0.auto_pause.0.enabled", "true"),
					resource.TestCheckResourceAttr("data.velodb_clusters.test", "clusters.0.auto_pause.0.idle_timeout_minutes", "15"),
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
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "public_endpoints.#", "2"),
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "public_endpoints.0.protocol", "jdbc"),
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "public_endpoints.0.host", "mock.selectdbcloud.com"),
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "public_endpoints.0.port", "9030"),
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "private_endpoints.#", "1"),
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "private_endpoints.0.endpoint_id", "vpce-mock"),
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "compute_clusters.#", "1"),
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "compute_clusters.0.cluster_id", "CL-MOCK-001"),
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "endpoint_service_name", "com.amazonaws.vpce.cn-beijing.vpce-svc-mock"),
				),
			},
		},
	})
}

func TestAccPublicAccessPolicyAllowAllDenyAllClearsRules(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
resource "velodb_warehouse_public_access_policy" "test" {
  warehouse_id = "WH-MOCK-001"
  policy       = "ALLOWLIST_ONLY"

  rules = [
    {
      cidr        = "203.0.113.10/32"
      description = "terraform-e2e"
    }
  ]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("velodb_warehouse_public_access_policy.test", "policy", "ALLOWLIST_ONLY"),
					resource.TestCheckResourceAttr("velodb_warehouse_public_access_policy.test", "rules.#", "1"),
				),
			},
			{
				Config: testProviderConfig(ts) + `
resource "velodb_warehouse_public_access_policy" "test" {
  warehouse_id = "WH-MOCK-001"
  policy       = "ALLOW_ALL"
}
`,
				Check: resource.TestCheckResourceAttr("velodb_warehouse_public_access_policy.test", "policy", "ALLOW_ALL"),
			},
			{
				Config: testProviderConfig(ts) + `
resource "velodb_warehouse_public_access_policy" "test" {
  warehouse_id = "WH-MOCK-001"
  policy       = "DENY_ALL"
}
`,
				Check: resource.TestCheckResourceAttr("velodb_warehouse_public_access_policy.test", "policy", "DENY_ALL"),
			},
		},
	})
}
