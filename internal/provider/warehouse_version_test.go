package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestWarehouseCoreVersionLifecycle(t *testing.T) {
	for _, selector := range []string{"4.1", "4.1.5", "legacy_id"} {
		t.Run(selector, func(t *testing.T) {
			ts := mockAPIServer(t)
			defer ts.Close()
			original := ts.Config.Handler
			var mu sync.Mutex
			actual := "4.1.5"
			creates, deletes, upgrades := 0, 0, 0
			ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				switch {
				case r.URL.Path == "/v1/warehouses" && r.Method == http.MethodPost:
					creates++
					data, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
						w.WriteHeader(500)
						return
					}
					r.Body = io.NopCloser(bytes.NewReader(data))
					var request client.CreateWarehouseRequest
					if err := json.Unmarshal(data, &request); err != nil {
						t.Error(err)
						w.WriteHeader(500)
						return
					}
					want := selector
					if strings.HasPrefix(want, "legacy") {
						want = "4.1.5"
					}
					if request.Version == nil || *request.Version != want {
						t.Errorf("creation version=%v want %s", request.Version, want)
					}
				case strings.HasSuffix(r.URL.Path, "/versions"):
					writeJSONResponse(t, w, map[string]any{"success": true, "data": []map[string]any{{"version": "4.1.9", "versionId": 9}, {"version": "4.2.3", "versionId": 23}}})
					return
				case strings.HasSuffix(r.URL.Path, "/settings/upgrade"):
					var request client.UpgradeWarehouseRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
						w.WriteHeader(400)
						return
					}
					if request.TargetVersionID != 9 {
						t.Errorf("target=%d want 9", request.TargetVersionID)
						w.WriteHeader(400)
						return
					}
					upgrades++
					actual = "4.1.9"
					writeJSONResponse(t, w, map[string]any{"success": true, "data": map[string]any{}})
					return
				case r.URL.Path == "/v1/warehouses/WH-MOCK-001" && r.Method == http.MethodGet:
					recorder := httptest.NewRecorder()
					original.ServeHTTP(recorder, r)
					var response map[string]any
					if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
						t.Error(err)
						w.WriteHeader(500)
						return
					}
					if data, ok := response["data"].(map[string]any); ok {
						data["coreVersion"] = actual
					}
					w.WriteHeader(recorder.Code)
					writeJSONResponse(t, w, response)
					return
				case r.Method == http.MethodDelete && r.URL.Path == "/v1/warehouses/WH-MOCK-001":
					deletes++
				}
				original.ServeHTTP(w, r)
			})
			config := func(version string) string {
				return testProviderConfig(ts) + fmt.Sprintf(`
resource "velodb_warehouse" "test" {
 name = "mock-warehouse"
 deployment_mode = "SaaS"
 cloud_provider = "aws"
 region = "us-east-1"
 admin_password = "TestPass@123"
 %s
 initial_cluster {
  zone = "us-east-1a"
  compute_vcpu = 4
  cache_gb = 100
 }
}
`, version)
			}
			initial := fmt.Sprintf("core_version = %q", selector)
			initialState := selector
			if strings.HasPrefix(selector, "legacy") {
				initial = `core_version = "4.1.5"`
				initialState = "4.1.5"
			}
			upgraded := config(`core_version = "4.1.9"`)
			if selector == "legacy_id" {
				upgraded = config(`core_version_id = 9`)
			}
			resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts), Steps: []resource.TestStep{
				{Config: config(`initial_core_version = "4.1.5"`), PlanOnly: true, ExpectError: regexp.MustCompile("Unsupported argument")},
				{Config: config(initial), Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("velodb_warehouse.test", "core_version", initialState))},
				{Config: config(initial), PlanOnly: true},
				{Config: config(""), Check: resource.TestCheckResourceAttr("velodb_warehouse.test", "core_version", "4.1.5")},
				{Config: config(""), PlanOnly: true},
				{Config: config(initial)},
				{Config: upgraded, Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("velodb_warehouse.test", "id", "WH-MOCK-001"),
					resource.TestCheckResourceAttr("velodb_warehouse.test", "core_version", "4.1.9"))},
				{Config: upgraded, PlanOnly: true},
				{Config: config(`core_version = "4.1.5"`), PlanOnly: true, ExpectError: regexp.MustCompile("downgrade")},
				{Config: config(`core_version = "4.2"`), PlanOnly: true, ExpectError: regexp.MustCompile("exact major.minor.patch")},
				{Config: config("core_version = \"4.1.9\"\n core_version_id = 9"), PlanOnly: true, ExpectError: regexp.MustCompile("Conflicting core version selectors")},
				{Config: config(`core_version = "4.1.99"`), ExpectError: regexp.MustCompile("not an available upgrade target")},
				{Config: upgraded, PlanOnly: true},
			}})
			mu.Lock()
			defer mu.Unlock()
			if creates != 1 || deletes != 1 || upgrades != 1 {
				t.Fatalf("creates=%d deletes=%d upgrades=%d", creates, deletes, upgrades)
			}
		})
	}
}
