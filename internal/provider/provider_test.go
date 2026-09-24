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
	byocCredentialDeleted := false
	byocNetworkDeleted := false
	byocWarehouseCreated := false
	byocWarehouseDeleted := false
	byocNetworkZoneMappings := []map[string]any{{"zoneId": "us-east-1a", "subnetId": "subnet-aaa"}}
	publicPolicy := "DENY_ALL"
	publicPolicyRules := []map[string]any{}

	// -- BYOC discovery endpoints --
	mux.HandleFunc("/v1/organization", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true, "requestId": "mock-organization",
			"data": map[string]any{
				"organizationId": "o-mock", "organizationName": "Mock Organization",
				"awsExternalId": "194819e8-e41d-afcb-7269-bcead94c1b55",
			},
		})
	})

	mux.HandleFunc("/v1/cloud-providers/aws/regions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if got := r.URL.Query().Get("deploymentMode"); got != "BYOC" {
			t.Errorf("expected deploymentMode=BYOC, got %q", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success": true, "requestId": "mock-byoc-regions",
			"data": []map[string]any{{
				"region": "us-east-1", "displayName": "US East (N. Virginia)",
				"endpointServiceId":   "vpce-svc-00c31b6d080ccb272",
				"endpointServiceName": "com.amazonaws.vpce.us-east-1.vpce-svc-00c31b6d080ccb272",
				"multiAzSupported":    true, "supportedDeploymentModes": []string{"BYOC"},
				"zones": []map[string]any{{"zone": "us-east-1a", "displayName": "US East (N. Virginia) A"}},
			}},
		})
	})

	// -- BYOC credential and network registration endpoints --
	mux.HandleFunc("/v1/cloud-settings/aws/credentials", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode credential request: %v", err)
		}
		if body["name"] != "production-credential" || body["region"] != "us-east-1" || body["bucketName"] != "velodb-data" {
			t.Errorf("unexpected credential request: %#v", body)
		}
		byocCredentialDeleted = false
		json.NewEncoder(w).Encode(map[string]any{
			"success": true, "requestId": "mock-create-credential",
			"data": map[string]any{"credentialId": 123},
		})
	})

	mux.HandleFunc("/v1/cloud-settings/aws/credentials/123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if byocCredentialDeleted && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"success": false, "code": "CredentialNotFound", "message": "not found"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{
				"success": true, "requestId": "mock-get-credential",
				"data": map[string]any{
					"credentialId": 123, "name": "production-credential", "cloudProvider": "aws", "region": "us-east-1",
					"bucketName": "velodb-data", "dataCredentialArn": "arn:aws:iam::111122223333:instance-profile/velodb-data",
					"deploymentCredentialArn": "arn:aws:iam::111122223333:role/velodb-deployment",
					"externalId":              "external-123", "warehouseCount": 0, "warehouseIdList": []string{},
					"createdAt": now.Format(time.RFC3339), "updatedAt": now.Format(time.RFC3339),
				},
			})
		case http.MethodDelete:
			if byocWarehouseCreated && (!byocWarehouseDeleted || !byocNetworkDeleted) {
				t.Error("credential configuration deleted before its warehouse and network dependencies")
			}
			byocCredentialDeleted = true
			json.NewEncoder(w).Encode(map[string]any{"success": true, "requestId": "mock-delete-credential", "data": map[string]any{}})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/cloud-settings/aws/network-configs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			CredentialID int64 `json:"credentialId"`
			ZoneMappings []struct {
				ZoneID   string `json:"zoneId"`
				SubnetID string `json:"subnetId"`
			} `json:"zoneMappings"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode network request: %v", err)
		}
		if body.CredentialID != 123 || (len(body.ZoneMappings) != 1 && len(body.ZoneMappings) != 3) {
			t.Errorf("unexpected network request: %#v", body)
		}
		byocNetworkZoneMappings = make([]map[string]any, 0, len(body.ZoneMappings))
		for _, mapping := range body.ZoneMappings {
			byocNetworkZoneMappings = append(byocNetworkZoneMappings, map[string]any{"zoneId": mapping.ZoneID, "subnetId": mapping.SubnetID})
		}
		byocNetworkDeleted = false
		json.NewEncoder(w).Encode(map[string]any{
			"success": true, "requestId": "mock-create-network",
			"data": map[string]any{"networkConfigId": 456},
		})
	})

	mux.HandleFunc("/v1/cloud-settings/aws/network-configs/456", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if byocNetworkDeleted && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"success": false, "code": "NetworkConfigNotFound", "message": "not found"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{
				"success": true, "requestId": "mock-get-network",
				"data": map[string]any{
					"networkConfigId": 456, "name": "production-network", "cloudProvider": "aws", "region": "us-east-1",
					"vpcId": "vpc-123", "zoneMappings": byocNetworkZoneMappings,
					"securityGroupId": "sg-123", "endpointId": "vpce-123", "warehouseCount": 0, "warehouseIdList": []string{},
					"createdAt": now.Format(time.RFC3339), "updatedAt": now.Format(time.RFC3339),
				},
			})
		case http.MethodDelete:
			if byocWarehouseCreated && !byocWarehouseDeleted {
				t.Error("network configuration deleted before its warehouse dependency")
			}
			byocNetworkDeleted = true
			json.NewEncoder(w).Encode(map[string]any{"success": true, "requestId": "mock-delete-network", "data": map[string]any{}})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// -- Warehouse endpoints --
	mux.HandleFunc("/v1/warehouses", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			data := []map[string]any{}
			if r.URL.Query().Get("warehouseId") == "WH-BYOC-LIST" {
				data = append(data, map[string]any{
					"warehouseId": "WH-BYOC-LIST", "name": "mock-byoc-list", "status": "Running",
					"cloudProvider": "aws", "region": "us-east-1", "zone": "us-east-1a",
					"deploymentMode": "BYOC", "coreVersion": "26.0.2", "payType": "PostPaid",
					"createdAt": now.Format(time.RFC3339),
				})
			} else if !whDeleted {
				data = append(data, map[string]any{
					"warehouseId": "WH-MOCK-001", "name": "mock-warehouse", "status": "Running",
					"cloudProvider": "aws", "region": "us-east-1", "zone": "us-east-1a",
					"deploymentMode": "SaaS", "coreVersion": "3.0.3", "payType": "PostPaid",
					"createdAt": now.Format(time.RFC3339),
				})
			}
			json.NewEncoder(w).Encode(map[string]any{
				"success": true, "requestId": "mock-list-wh", "page": 1, "size": 20,
				"total": len(data), "data": data,
			})
		case http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode warehouse request: %v", err)
			}
			if body["deploymentMode"] == "BYOC" {
				if body["cloudProvider"] != "aws" || body["setupMode"] != "advanced" || body["credentialId"] != float64(123) || body["networkConfigId"] != float64(456) {
					t.Errorf("unexpected advanced BYOC warehouse request: %#v", body)
				}
				byocWarehouseCreated = true
				byocWarehouseDeleted = false
				json.NewEncoder(w).Encode(map[string]any{
					"success": true, "requestId": "mock-create-byoc-wh",
					"data": map[string]any{"warehouseId": "WH-BYOC-CREATE-001"},
				})
				return
			}
			whDeleted = false
			json.NewEncoder(w).Encode(map[string]any{
				"success": true, "requestId": "mock-create-wh",
				"data": map[string]any{"warehouseId": "WH-MOCK-001"},
			})
		default:
			w.WriteHeader(405)
		}
	})

	mux.HandleFunc("/v1/warehouses/WH-BYOC-CREATE-001", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if byocWarehouseDeleted && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"success": false, "code": "WarehouseNotFound", "message": "not found"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{
				"success": true, "requestId": "mock-get-created-byoc-wh",
				"data": map[string]any{
					"warehouseId": "WH-BYOC-CREATE-001", "name": "advanced-byoc", "status": "Running",
					"cloudProvider": "aws", "region": "us-east-1", "zone": "us-east-1a",
					"deploymentMode": "BYOC", "coreVersion": "3.0.3", "payType": "PostPaid",
					"createdAt": now.Format(time.RFC3339),
				},
			})
		case http.MethodDelete:
			byocWarehouseDeleted = true
			json.NewEncoder(w).Encode(map[string]any{"success": true, "requestId": "mock-delete-created-byoc-wh", "data": map[string]any{}})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/warehouses/WH-BYOC-CREATE-001/clusters", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true, "requestId": "mock-list-created-byoc-cl", "page": 1, "size": 20, "total": 1,
			"data": []map[string]any{{
				"clusterId": "CL-BYOC-CREATE-001", "warehouseId": "WH-BYOC-CREATE-001",
				"name": "initial_cluster", "status": "Running", "clusterType": "COMPUTE",
				"cloudProvider": "aws", "region": "us-east-1", "zone": "us-east-1a",
			}},
		})
	})

	mux.HandleFunc("/v1/warehouses/WH-BYOC-LIST", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"code": "WarehouseNotFound", "message": "not found", "success": false, "requestId": "mock-byoc-list-not-found",
		})
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
					"cloudProvider": "aws", "region": "us-east-1", "zone": "us-east-1a",
					"deploymentMode": "SaaS", "coreVersion": "3.0.3", "payType": "PostPaid",
					"endpointServiceId": "vpce-svc-mock", "endpointServiceName": "com.amazonaws.vpce.us-east-1.vpce-svc-mock",
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

	mux.HandleFunc("/v1/warehouses/WH-BYOC-LIST/clusters", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success": true, "requestId": "mock-list-byoc-list-cl", "page": 1, "size": 20,
			"total": 1,
			"data": []map[string]any{{
				"clusterId": "CL-BYOC-LIST", "warehouseId": "WH-BYOC-LIST",
				"name": "byoc_list_cluster", "status": "Running", "clusterType": "COMPUTE",
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

	mux.HandleFunc("/v1/private-link/endpoint-services", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		data := []map[string]any{
			{
				"cloudProvider":       "aws",
				"region":              "us-east-1",
				"zone":                "us-east-1a",
				"endpointServiceId":   "vpce-svc-outbound-001",
				"endpointServiceName": "com.amazonaws.vpce.us-east-1.vpce-svc-outbound-001",
				"providerAccountId":   "123456789012",
				"description":         "mock outbound service",
				"connected":           true,
				"createdAt":           now.Format(time.RFC3339),
				"endpoints": []map[string]any{{
					"endpointId":   "vpce-outbound-001",
					"endpointName": "endpoint-mock-outbound",
					"domain":       "vpce-outbound-001.example.vpce.amazonaws.com",
					"status":       "available",
					"createdAt":    now.Format(time.RFC3339),
				}},
			},
			{
				"cloudProvider":       "aws",
				"region":              "us-east-1",
				"zone":                "us-east-1a",
				"endpointServiceId":   "eps-mock-002",
				"endpointServiceName": "com.amazonaws.vpce.us-east-1.eps-mock-002",
				"description":         "other outbound service",
				"connected":           false,
				"createdAt":           now.Format(time.RFC3339),
			},
		}

		filtered := make([]map[string]any, 0, len(data))
		for _, svc := range data {
			if q := r.URL.Query().Get("cloudProvider"); q != "" && svc["cloudProvider"] != q {
				continue
			}
			if q := r.URL.Query().Get("region"); q != "" && svc["region"] != q {
				continue
			}
			filtered = append(filtered, svc)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"success": true, "requestId": "mock-list-endpoint-services",
			"data": filtered,
		})
	})

	// -- Cluster endpoints --
	clusterData := func() map[string]any {
		return map[string]any{
			"clusterId": "CL-MOCK-001", "warehouseId": "WH-MOCK-001",
			"name": "mock_cluster", "status": "Running", "clusterType": "COMPUTE",
			"cloudProvider": "aws", "region": "us-east-1", "zone": "us-east-1a",
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
  cloud_provider  = "aws"
  region          = "us-east-1"

  admin_password         = "TestPass@123"
  admin_password_version = 1

  initial_cluster {
    zone         = "us-east-1a"
    compute_vcpu = 4
    ratio        = 8
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
					resource.TestCheckResourceAttr("velodb_warehouse.test", "initial_cluster.0.ratio", "8"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "cloud_provider", "aws"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "region", "us-east-1"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "deployment_mode", "SaaS"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "core_version", "3.0.3"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "endpoint_service_name", "com.amazonaws.vpce.us-east-1.vpce-svc-mock"),
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
  cloud_provider  = "aws"
  region          = "us-east-1"

  admin_password         = "TestPass@123"
  admin_password_version = 1

  initial_cluster {
    zone         = "us-east-1a"
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

func TestWarehouseImportBYOCFallsBackToList(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
resource "velodb_warehouse" "byoc" {
  name            = "mock-byoc-list"
  deployment_mode = "BYOC"
  cloud_provider  = "aws"
  region          = "us-east-1"
}
`,
				ResourceName:            "velodb_warehouse.byoc",
				ImportState:             true,
				ImportStateId:           "WH-BYOC-LIST",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("velodb_warehouse.byoc", "id", "WH-BYOC-LIST"),
					resource.TestCheckResourceAttr("velodb_warehouse.byoc", "name", "mock-byoc-list"),
					resource.TestCheckResourceAttr("velodb_warehouse.byoc", "deployment_mode", "BYOC"),
					resource.TestCheckResourceAttr("velodb_warehouse.byoc", "initial_cluster_id", "CL-BYOC-LIST"),
				),
			},
		},
	})
}

func TestAccWarehouseAdvancedBYOC(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{{
			Config: testProviderConfig(ts) + `
resource "velodb_byoc_credential" "test" {
  cloud_provider            = "aws"
  name                      = "production-credential"
  region                    = "us-east-1"
  bucket_name               = "velodb-data"
  data_credential_arn       = "arn:aws:iam::111122223333:instance-profile/velodb-data"
  deployment_credential_arn = "arn:aws:iam::111122223333:role/velodb-deployment"
}

resource "velodb_byoc_network" "test" {
  cloud_provider    = "aws"
  name              = "production-network"
  credential_id     = velodb_byoc_credential.test.id
  security_group_id = "sg-123"
  endpoint_id       = "vpce-123"
  zone_mappings = [{
    zone_id   = "us-east-1a"
    subnet_id = "subnet-aaa"
  }]
}

resource "velodb_warehouse" "test" {
  name              = "advanced-byoc"
  deployment_mode   = "BYOC"
  cloud_provider    = "aws"
  region            = "us-east-1"
  setup_mode        = "advanced"
  credential_id     = velodb_byoc_credential.test.id
  network_config_id = velodb_byoc_network.test.id
  admin_password    = "TestPass@123"
  tags               = { environment = "test" }

  initial_cluster {
    zone         = "us-east-1a"
    compute_vcpu = 8
    cache_gb     = 400
  }
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("velodb_byoc_credential.test", "id", "123"),
				resource.TestCheckResourceAttr("velodb_byoc_network.test", "id", "456"),
				resource.TestCheckResourceAttr("velodb_warehouse.test", "id", "WH-BYOC-CREATE-001"),
				resource.TestCheckResourceAttr("velodb_warehouse.test", "deployment_mode", "BYOC"),
				resource.TestCheckResourceAttr("velodb_warehouse.test", "setup_mode", "advanced"),
				resource.TestCheckResourceAttr("velodb_warehouse.test", "credential_id", "123"),
				resource.TestCheckResourceAttr("velodb_warehouse.test", "network_config_id", "456"),
				resource.TestCheckResourceAttr("velodb_warehouse.test", "initial_cluster_id", "CL-BYOC-CREATE-001"),
			),
		}},
	})
}

func TestWarehouseCreateBYOCRejectsGuided(t *testing.T) {
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
				ExpectError: regexp.MustCompile("Only advanced BYOC setup is supported"),
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
  cloud_provider  = "aws"
  region          = "us-east-1"

  admin_password         = "TestPass@123"
  admin_password_version = 1

  initial_cluster {
    zone         = "us-east-1a"
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
  cloud_provider = "aws"
  region         = "us-east-1"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "total", "1"),
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "warehouses.#", "1"),
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "warehouses.0.warehouse_id", "WH-MOCK-001"),
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "warehouses.0.name", "mock-warehouse"),
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "warehouses.0.status", "Running"),
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "warehouses.0.endpoint_service_id", "vpce-svc-mock"),
					resource.TestCheckResourceAttr("data.velodb_warehouses.test", "warehouses.0.endpoint_service_name", "com.amazonaws.vpce.us-east-1.vpce-svc-mock"),
				),
			},
		},
	})
}

func TestAccBYOCPrerequisitesDataSource(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{{
			Config: testProviderConfig(ts) + `
data "velodb_byoc_prerequisites" "test" {
  cloud_provider = "aws"
  region         = "us-east-1"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.velodb_byoc_prerequisites.test", "cloud_provider", "aws"),
				resource.TestCheckResourceAttr("data.velodb_byoc_prerequisites.test", "region", "us-east-1"),
				resource.TestCheckResourceAttr("data.velodb_byoc_prerequisites.test", "external_id", "194819e8-e41d-afcb-7269-bcead94c1b55"),
				resource.TestCheckResourceAttr("data.velodb_byoc_prerequisites.test", "deployment_assumer_role_arn", "arn:aws:iam::757278738533:role/VeloDBDeploymentAssumer"),
				resource.TestCheckResourceAttr("data.velodb_byoc_prerequisites.test", "endpoint_service_id", "vpce-svc-00c31b6d080ccb272"),
				resource.TestCheckResourceAttr("data.velodb_byoc_prerequisites.test", "endpoint_service_name", "com.amazonaws.vpce.us-east-1.vpce-svc-00c31b6d080ccb272"),
				resource.TestCheckResourceAttr("data.velodb_byoc_prerequisites.test", "multi_az_supported", "true"),
				resource.TestCheckResourceAttr("data.velodb_byoc_prerequisites.test", "zones.#", "1"),
				resource.TestCheckResourceAttr("data.velodb_byoc_prerequisites.test", "zones.0.zone", "us-east-1a"),
			),
		}},
	})
}

func TestAccAWSPolicyDataSources(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{{
			Config: testProviderConfig(ts) + `
data "velodb_aws_assume_role_policy" "test" {
  principal_arn = "arn:aws:iam::757278738533:role/VeloDBDeploymentAssumer"
  external_id   = "external-123"
}

data "velodb_aws_crossaccount_policy" "test" {
  bucket_name        = "velodb-data"
  data_credential_arn = "arn:aws:iam::111122223333:instance-profile/velodb-data"
}

data "velodb_aws_data_access_assume_role_policy" "test" {
  role_arn = "arn:aws:iam::111122223333:role/velodb-data"
}

data "velodb_aws_data_access_policy" "test" {
  bucket_name = "velodb-data"
  role_arn    = "arn:aws:iam::111122223333:role/velodb-data"
  tde_kms_arn = "arn:aws:kms:us-east-1:111122223333:key/12345678-1234-1234-1234-123456789012"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				checkAttributeContains("data.velodb_aws_assume_role_policy.test", "json", "external-123"),
				checkAttributeContains("data.velodb_aws_crossaccount_policy.test", "json", "iam:PassRole"),
				checkAttributeContains("data.velodb_aws_data_access_assume_role_policy.test", "json", "ec2.amazonaws.com"),
				checkAttributeContains("data.velodb_aws_data_access_policy.test", "json", "KMSAccess"),
			),
		}},
	})
}

func checkAttributeContains(resourceName, attribute, want string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(resourceName, attribute, func(value string) error {
		if !strings.Contains(value, want) {
			return fmt.Errorf("%s.%s does not contain %q", resourceName, attribute, want)
		}
		return nil
	})
}

func TestAccBYOCCredentialResource(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
resource "velodb_byoc_credential" "test" {
  cloud_provider            = "aws"
  name                      = "production-credential"
  region                    = "us-east-1"
  bucket_name               = "velodb-data"
  data_credential_arn       = "arn:aws:iam::111122223333:instance-profile/velodb-data"
  deployment_credential_arn = "arn:aws:iam::111122223333:role/velodb-deployment"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("velodb_byoc_credential.test", "id", "123"),
					resource.TestCheckResourceAttr("velodb_byoc_credential.test", "external_id", "external-123"),
					resource.TestCheckResourceAttr("velodb_byoc_credential.test", "warehouse_count", "0"),
					resource.TestCheckResourceAttr("velodb_byoc_credential.test", "warehouse_ids.#", "0"),
				),
			},
			{
				ResourceName:      "velodb_byoc_credential.test",
				ImportState:       true,
				ImportStateId:     "aws/123",
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccBYOCCredentialRejectsInvalidARNTypes(t *testing.T) {
	tests := []struct {
		name          string
		dataARN       string
		deploymentARN string
		want          string
	}{
		{
			name:          "role used as data credential",
			dataARN:       "arn:aws:iam::111122223333:role/velodb-data",
			deploymentARN: "arn:aws:iam::111122223333:role/velodb-deployment",
			want:          "must be a commercial AWS IAM instance-profile ARN",
		},
		{
			name:          "instance profile used as deployment credential",
			dataARN:       "arn:aws:iam::111122223333:instance-profile/velodb-data",
			deploymentARN: "arn:aws:iam::111122223333:instance-profile/velodb-deployment",
			want:          "must be a commercial AWS IAM role ARN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := mockAPIServer(t)
			defer ts.Close()

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
				Steps: []resource.TestStep{{
					Config: testProviderConfig(ts) + fmt.Sprintf(`
resource "velodb_byoc_credential" "test" {
  cloud_provider            = "aws"
  name                      = "production-credential"
  region                    = "us-east-1"
  bucket_name               = "velodb-data"
  data_credential_arn       = %q
  deployment_credential_arn = %q
}
`, tt.dataARN, tt.deploymentARN),
					ExpectError: regexp.MustCompile(tt.want),
				}},
			})
		})
	}
}

func TestAccBYOCNetworkResource(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
resource "velodb_byoc_network" "test" {
  cloud_provider    = "aws"
  name              = "production-network"
  credential_id     = 123
  security_group_id = "sg-123"
  endpoint_id       = "vpce-123"

  zone_mappings = [{
    zone_id   = "us-east-1a"
    subnet_id = "subnet-aaa"
  }]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("velodb_byoc_network.test", "id", "456"),
					resource.TestCheckResourceAttr("velodb_byoc_network.test", "region", "us-east-1"),
					resource.TestCheckResourceAttr("velodb_byoc_network.test", "vpc_id", "vpc-123"),
					resource.TestCheckResourceAttr("velodb_byoc_network.test", "zone_mappings.#", "1"),
					resource.TestCheckResourceAttr("velodb_byoc_network.test", "zone_mappings.0.zone_id", "us-east-1a"),
				),
			},
			{
				ResourceName:      "velodb_byoc_network.test",
				ImportState:       true,
				ImportStateId:     "aws/456/123",
				ImportStateVerify: true,
			},
		},
	})
}

func TestBYOCNetworkRejectsTwoZoneMappings(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{{
			Config: testProviderConfig(ts) + `
resource "velodb_byoc_network" "test" {
  cloud_provider = "aws"
  name = "production-network"
  credential_id = 123
  security_group_id = "sg-123"
  zone_mappings = [
    { zone_id = "us-east-1a", subnet_id = "subnet-aaa" },
    { zone_id = "us-east-1b", subnet_id = "subnet-bbb" }
  ]
}
`,
			ExpectError: regexp.MustCompile("Invalid number of zone mappings"),
		}},
	})
}

func TestAccBYOCNetworkResourceAcceptsThreeZoneMappings(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{{
			Config: testProviderConfig(ts) + `
resource "velodb_byoc_network" "test" {
  cloud_provider    = "aws"
  name              = "production-network"
  credential_id     = 123
  security_group_id = "sg-123"
  endpoint_id       = "vpce-123"
  zone_mappings = [
    { zone_id = "us-east-1a", subnet_id = "subnet-aaa" },
    { zone_id = "us-east-1b", subnet_id = "subnet-bbb" },
    { zone_id = "us-east-1c", subnet_id = "subnet-ccc" }
  ]
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("velodb_byoc_network.test", "zone_mappings.#", "3"),
				resource.TestCheckResourceAttr("velodb_byoc_network.test", "zone_mappings.2.zone_id", "us-east-1c"),
			),
		}},
	})
}

func TestBYOCNetworkRejectsDuplicateZoneOrSubnet(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{{
			Config: testProviderConfig(ts) + `
resource "velodb_byoc_network" "test" {
  cloud_provider = "aws"
  name = "production-network"
  credential_id = 123
  security_group_id = "sg-123"
  zone_mappings = [
    { zone_id = "us-east-1a", subnet_id = "subnet-aaa" },
    { zone_id = "us-east-1a", subnet_id = "subnet-bbb" },
    { zone_id = "us-east-1c", subnet_id = "subnet-aaa" }
  ]
}
`,
			ExpectError: regexp.MustCompile("Duplicate Availability Zone|Duplicate subnet"),
		}},
	})
}

func TestBYOCPrerequisitesRejectsUnavailableRegion(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{{
			Config: testProviderConfig(ts) + `
data "velodb_byoc_prerequisites" "test" {
  cloud_provider = "aws"
  region         = "eu-west-1"
}
`,
			ExpectError: regexp.MustCompile("AWS BYOC region is not available"),
		}},
	})
}

func TestBYOCPrerequisitesRejectsUnsupportedProvider(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{{
			Config: testProviderConfig(ts) + `
data "velodb_byoc_prerequisites" "test" {
  cloud_provider = "gcp"
  region         = "us-east1"
}
`,
			ExpectError: regexp.MustCompile(`value must be one of: \["aws"\]`),
		}},
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
					resource.TestCheckResourceAttr("data.velodb_warehouse_connections.test", "endpoint_service_name", "com.amazonaws.vpce.us-east-1.vpce-svc-mock"),
				),
			},
		},
	})
}

func TestAccPrivateLinkEndpointServicesDataSource(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(ts) + `
data "velodb_private_link_endpoint_services" "test" {
  cloud_provider      = "aws"
  region              = "us-east-1"
  endpoint_service_id = "vpce-svc-outbound-001"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "total", "1"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.#", "1"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.cloud_provider", "aws"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.region", "us-east-1"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.zone", "us-east-1a"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.endpoint_service_id", "vpce-svc-outbound-001"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.endpoint_service_name", "com.amazonaws.vpce.us-east-1.vpce-svc-outbound-001"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.provider_account_id", "123456789012"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.description", "mock outbound service"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.connected", "true"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.created_at", "2026-04-06T10:30:00Z"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.endpoints.#", "1"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.endpoints.0.endpoint_id", "vpce-outbound-001"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.endpoints.0.endpoint_name", "endpoint-mock-outbound"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.endpoints.0.domain", "vpce-outbound-001.example.vpce.amazonaws.com"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.endpoints.0.status", "available"),
					resource.TestCheckResourceAttr("data.velodb_private_link_endpoint_services.test", "services.0.endpoints.0.created_at", "2026-04-06T10:30:00Z"),
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
