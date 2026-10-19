package provider

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func warehouseImportCheck(want map[string]string) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if len(states) != 1 {
			return fmt.Errorf("expected one imported warehouse, got %d", len(states))
		}
		for name, value := range want {
			if got := states[0].Attributes[name]; got != value {
				return fmt.Errorf("imported %s=%q, want %q", name, got, value)
			}
		}
		return nil
	}
}

func TestAccWarehouseInitialClusterImportDoesNotGuess(t *testing.T) {
	ts := mockAPIServer(t)
	defer ts.Close()
	original := ts.Config.Handler
	var mu sync.Mutex
	lists, creates, deletes := 0, 0, 0
	clusters := []client.ClusterItem{
		{ClusterID: "c-newer", WarehouseID: "WH-MOCK-001", ClusterType: "COMPUTE"},
		{ClusterID: "c-original", WarehouseID: "WH-MOCK-001", ClusterType: "COMPUTE"},
	}
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case req.URL.Path == "/v1/warehouses/WH-MOCK-001/clusters":
			lists++
			writeJSONResponse(t, w, client.PageResponse[client.ClusterItem]{Success: true, Data: clusters, Total: int64(len(clusters))})
			return
		case req.Method == http.MethodPost && req.URL.Path == "/v1/warehouses":
			creates++
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/warehouses/WH-MOCK-001":
			deletes++
		}
		original.ServeHTTP(w, req)
	})
	config := testProviderConfig(ts) + `
resource "velodb_warehouse" "test" {
  name            = "mock-warehouse"
  deployment_mode = "SaaS"
  cloud_provider  = "aws"
  region          = "us-east-1"
}
`
	updated := strings.Replace(config, `name            = "mock-warehouse"`, "name = \"mock-warehouse\"\n  admin_password_version = 1", 1)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{
				Config:             config,
				ResourceName:       "velodb_warehouse.test",
				ImportState:        true,
				ImportStateId:      "WH-MOCK-001",
				ImportStatePersist: true,
				ImportStateCheck:   warehouseImportCheck(map[string]string{"id": "WH-MOCK-001", "initial_cluster_id": ""}),
			},
			{Config: config, Check: resource.TestCheckNoResourceAttr("velodb_warehouse.test", "initial_cluster_id")},
			{Config: updated, Check: resource.TestCheckNoResourceAttr("velodb_warehouse.test", "initial_cluster_id")},
			{Config: updated, PlanOnly: true},
		},
	})
	mu.Lock()
	defer mu.Unlock()
	if lists != 0 || creates != 0 || deletes != 1 {
		t.Fatalf("import/read/update must not create warehouses or discover a cluster: lists=%d creates=%d deletes=%d", lists, creates, deletes)
	}
}
