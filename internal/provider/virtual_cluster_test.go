package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

type virtualClusterMock struct {
	mu                                   sync.Mutex
	current                              client.VirtualCluster
	exists, pendingCreate, pendingDelete bool
	creates, renames, updates, deletes   int
	clusters                             map[string]client.ClusterDetail
	clusterCreates                       int
	events                               []string
	deleteRequestIDs                     []string
}

func newVirtualClusterMock(t *testing.T) (*httptest.Server, *virtualClusterMock) {
	t.Helper()
	m := &virtualClusterMock{clusters: map[string]client.ClusterDetail{}}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		base := "/v1/warehouses/w-test/virtual-clusters"
		if req.Header.Get("X-API-Key") != "test-api-key" || (req.Method != http.MethodGet && req.Header.Get("RequestId") == "") {
			t.Error("request missing authentication or write idempotency header")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if strings.HasPrefix(req.URL.Path, "/v1/warehouses/w-test/clusters") {
			m.servePhysicalCluster(t, w, req)
			return
		}
		if req.Method == http.MethodPost && req.URL.Path == base {
			if m.exists {
				t.Error("provider attempted to recreate an existing virtual cluster")
				w.WriteHeader(http.StatusConflict)
				return
			}
			var body client.CreateVirtualClusterRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Errorf("decode create: %v", err)
				return
			}
			m.current = client.VirtualCluster{
				ID: "v-test", WarehouseID: "w-test", Name: body.Name, Status: "Running",
				ActiveClusterID: body.ActiveClusterID, StandbyClusterID: body.StandbyClusterID,
				CreatedAt: "2026-10-13T08:00:00Z", UpdatedAt: "2026-10-13T08:00:00Z",
			}
			if m.pendingCreate {
				m.current.Status = "Creating"
			}
			m.exists = true
			m.creates++
			w.WriteHeader(http.StatusCreated)
			writeJSONResponse(t, w, map[string]any{"success": true, "data": client.CreateVirtualClusterResult{VirtualClusterID: m.current.ID}})
			return
		}
		if !strings.HasPrefix(req.URL.Path, base+"/v-test") {
			t.Errorf("unexpected virtual-cluster path: %s", req.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !m.exists {
			w.WriteHeader(http.StatusNotFound)
			writeJSONResponse(t, w, map[string]any{"code": "VirtualClusterNotFound", "message": "target absent"})
			return
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == base+"/v-test":
			writeJSONResponse(t, w, client.APIResponse[client.VirtualCluster]{Success: true, Data: m.current})
		case req.Method == http.MethodPost && req.URL.Path == base+"/v-test/rename":
			var body client.RenameVirtualClusterRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Errorf("decode rename: %v", err)
				return
			}
			m.current.Name = body.NewName
			m.renames++
			writeJSONResponse(t, w, map[string]any{"success": true})
		case req.Method == http.MethodPost && req.URL.Path == base+"/v-test/update":
			var body client.UpdateVirtualClusterRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Errorf("decode update: %v", err)
				return
			}
			m.current.ActiveClusterID, m.current.StandbyClusterID = body.NewActiveClusterID, body.NewStandbyClusterID
			m.updates++
			m.events = append(m.events, "pair:"+body.NewStandbyClusterID)
			writeJSONResponse(t, w, map[string]any{"success": true})
		case req.Method == http.MethodDelete && req.URL.Path == base+"/v-test":
			m.deletes++
			m.deleteRequestIDs = append(m.deleteRequestIDs, req.Header.Get("RequestId"))
			if m.pendingDelete {
				m.current.Status = "Deleting"
			} else {
				m.exists = false
			}
			writeJSONResponse(t, w, map[string]any{"success": true})
		default:
			t.Errorf("unexpected operation: %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	return ts, m
}

func virtualClusterConfig(ts *httptest.Server, name, active, standby, extra string) string {
	return testProviderConfig(ts) + fmt.Sprintf(`
resource "velodb_virtual_cluster" "test" {
  warehouse_id               = "w-test"
  name                       = %q
  desired_active_cluster_id  = %q
  desired_standby_cluster_id = %q
  %s
}
`, name, active, standby, extra)
}

func virtualClusterCheck(name, desiredActive, desiredStandby, active, standby string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "id", "v-test"),
		resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "warehouse_id", "w-test"),
		resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "name", name),
		resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "desired_active_cluster_id", desiredActive),
		resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "desired_standby_cluster_id", desiredStandby),
		resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "active_cluster_id", active),
		resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "standby_cluster_id", standby),
	)
}

func TestAccVirtualClusterLifecycle(t *testing.T) {
	ts, mock := newVirtualClusterMock(t)
	defer ts.Close()
	base := virtualClusterConfig(ts, "production", "c-a", "c-b", "")
	reconciled := virtualClusterConfig(ts, "renamed", "c-a", "c-b", "pair_reconcile_trigger = 1")
	replaced := virtualClusterConfig(ts, "renamed", "c-a", "c-c", "pair_reconcile_trigger = 1")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{Config: base, Check: resource.ComposeAggregateTestCheckFunc(
				virtualClusterCheck("production", "c-a", "c-b", "c-a", "c-b"),
				resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "status", "Running"),
				resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "at_risk", "false"),
				resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "risk_reasons.#", "0"),
				resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "created_at", "2026-10-13T08:00:00Z"),
			)},
			{ResourceName: "velodb_virtual_cluster.test", ImportState: true, ImportStateId: "w-test/v-test", ImportStateVerify: true},
			{PreConfig: func() {
				mock.mu.Lock()
				defer mock.mu.Unlock()
				mock.current.ActiveClusterID, mock.current.StandbyClusterID = "c-b", "c-a"
				mock.current.Status, mock.current.AtRisk = "AtRisk", true
				mock.current.RiskReasons = []string{"ComputeResourceLow"}
			}, Config: base, PlanOnly: true},
			{Config: virtualClusterConfig(ts, "renamed", "c-a", "c-b", ""), Check: resource.ComposeAggregateTestCheckFunc(
				virtualClusterCheck("renamed", "c-a", "c-b", "c-b", "c-a"),
				resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "at_risk", "true"),
				resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "risk_reasons.#", "1"),
			)},
			{Config: virtualClusterConfig(ts, "renamed", "c-b", "c-a", ""), Check: virtualClusterCheck("renamed", "c-b", "c-a", "c-b", "c-a")},
			{Config: reconciled, Check: virtualClusterCheck("renamed", "c-a", "c-b", "c-a", "c-b")},
			{Config: replaced, Check: virtualClusterCheck("renamed", "c-a", "c-c", "c-a", "c-c")},
			{Config: strings.Replace(replaced, `warehouse_id               = "w-test"`, `warehouse_id               = "w-other"`, 1), PlanOnly: true, ExpectError: regexp.MustCompile("warehouse cannot be changed")},
			{PreConfig: func() { mock.mu.Lock(); mock.current.Name = "external_name"; mock.mu.Unlock() }, Config: replaced, PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: replaced, Check: virtualClusterCheck("renamed", "c-a", "c-c", "c-a", "c-c")},
			{Config: virtualClusterConfig(ts, "renamed", "c-c", "c-d", "pair_reconcile_trigger = 1"), Check: virtualClusterCheck("renamed", "c-c", "c-d", "c-c", "c-d")},
		},
	})
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if mock.creates != 1 || mock.deletes != 1 || mock.renames != 2 || mock.updates != 3 || mock.exists {
		t.Fatalf("unexpected lifecycle writes: creates=%d deletes=%d renames=%d updates=%d exists=%t", mock.creates, mock.deletes, mock.renames, mock.updates, mock.exists)
	}
}

func TestAccVirtualClusterPendingCreate(t *testing.T) {
	ts, mock := newVirtualClusterMock(t)
	defer ts.Close()
	mock.pendingCreate = true
	config := virtualClusterConfig(ts, "production", "c-a", "c-b", `timeouts { create = "200ms" }`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
				virtualClusterCheck("production", "c-a", "c-b", "c-a", "c-b"),
				resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "status", "Creating"),
			)},
			{Config: config, PlanOnly: true},
			{PreConfig: func() { mock.mu.Lock(); mock.current.Status = "Running"; mock.mu.Unlock() }, Config: config, Check: resource.TestCheckResourceAttr("velodb_virtual_cluster.test", "status", "Running")},
		},
	})
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if mock.creates != 1 || mock.deletes != 1 || mock.exists {
		t.Fatalf("pending create must retain its ID without replacement: creates=%d deletes=%d exists=%t", mock.creates, mock.deletes, mock.exists)
	}
}

func TestAccVirtualClusterDeleteRetryRetainsState(t *testing.T) {
	ts, mock := newVirtualClusterMock(t)
	defer ts.Close()
	mock.pendingDelete = true
	config := virtualClusterConfig(ts, "production", "c-a", "c-b", `timeouts { delete = "200ms" }`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{Config: config, Check: virtualClusterCheck("production", "c-a", "c-b", "c-a", "c-b")},
			{Config: config, Destroy: true, ExpectError: regexp.MustCompile("timed out waiting for virtual-cluster absence")},
			{Config: config, PlanOnly: true},
			{PreConfig: func() { mock.mu.Lock(); mock.pendingDelete = false; mock.mu.Unlock() }, Config: config, Destroy: true},
		},
	})
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if mock.creates != 1 || mock.deletes != 2 || mock.exists {
		t.Fatalf("retry must resume deletion of the retained resource: creates=%d deletes=%d exists=%t", mock.creates, mock.deletes, mock.exists)
	}
	if mock.deleteRequestIDs[0] == mock.deleteRequestIDs[1] {
		t.Fatal("a new destroy attempt must use a new RequestId to avoid replaying the previous task acceptance")
	}
}
