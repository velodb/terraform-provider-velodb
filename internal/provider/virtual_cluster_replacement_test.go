package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

// The caller holds mu, shared with virtual-cluster updates so bound deletes fail.
func (m *virtualClusterMock) servePhysicalCluster(t *testing.T, w http.ResponseWriter, req *http.Request) {
	t.Helper()
	base := "/v1/warehouses/w-test/clusters"
	if req.Method == http.MethodPost && req.URL.Path == base {
		var body client.CreateClusterRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Errorf("decode physical create: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for _, cluster := range m.clusters {
			if strings.EqualFold(cluster.Name, body.Name) {
				w.WriteHeader(http.StatusConflict)
				writeJSONResponse(t, w, map[string]any{"code": "NameConflict", "message": "name already exists"})
				return
			}
		}
		m.clusterCreates++
		id := fmt.Sprintf("c-%d", m.clusterCreates)
		now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
		m.clusters[id] = client.ClusterDetail{
			ClusterItem: client.ClusterItem{
				ClusterID: id, WarehouseID: "w-test", Name: body.Name, Status: "Running",
				ClusterType: body.ClusterType, Zone: *body.Zone, CloudProvider: "aws",
				Region: "us-east-1", BillingModel: "on_demand", CreatedAt: &now,
			},
			BillingSummary: &client.ClusterBillingSummary{NodeCount: 1, TotalCpu: *body.ComputeVcpu, TotalDiskSizeGb: *body.CacheGb},
			BillingPools: &client.ClusterBillingPools{OnDemand: &client.ClusterBillingPool{
				NodeCount: 1, Cpu: *body.ComputeVcpu, DiskSizeGb: *body.CacheGb,
			}},
		}
		m.events = append(m.events, "create:"+id)
		writeJSONResponse(t, w, client.APIResponse[client.CreateClusterResult]{Success: true, Data: client.CreateClusterResult{ClusterID: id}})
		return
	}
	id := strings.TrimPrefix(req.URL.Path, base+"/")
	cluster, exists := m.clusters[id]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		writeJSONResponse(t, w, map[string]any{"code": "ClusterNotFound", "message": "target absent"})
		return
	}
	switch req.Method {
	case http.MethodGet:
		writeJSONResponse(t, w, client.APIResponse[client.ClusterDetail]{Success: true, Data: cluster})
	case http.MethodDelete:
		if m.exists && (m.current.ActiveClusterID == id || m.current.StandbyClusterID == id) {
			m.events = append(m.events, "blocked:"+id)
			w.WriteHeader(http.StatusConflict)
			writeJSONResponse(t, w, map[string]any{"code": "OperationConflict", "message": "cluster is bound to a virtual cluster"})
			return
		}
		delete(m.clusters, id)
		m.events = append(m.events, "delete:"+id)
		writeJSONResponse(t, w, map[string]any{"success": true})
	default:
		t.Errorf("unexpected physical-cluster operation: %s %s", req.Method, req.URL.Path)
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func physicalMemberConfig(address, name, zone string, createBeforeDestroy bool) string {
	lifecycle := ""
	if createBeforeDestroy {
		lifecycle = "lifecycle { create_before_destroy = true }"
	}
	return fmt.Sprintf(`
resource "velodb_cluster" %q {
  warehouse_id = "w-test"
  name         = %q
  cluster_type = "COMPUTE"
  zone         = %q
  compute_vcpu = 4
  cache_gb     = 100
  %s
}
`, address, name, zone, lifecycle)
}

func memberReplacementConfig(ts *httptest.Server, members, address string) string {
	return testProviderConfig(ts) + members + fmt.Sprintf(`
resource "velodb_virtual_cluster" "test" {
  warehouse_id               = "w-test"
  name                       = "production"
  desired_active_cluster_id  = "c-a"
  desired_standby_cluster_id = velodb_cluster.%s.id
}
`, address)
}

func TestAccVirtualClusterMemberReplacement(t *testing.T) {
	old := physicalMemberConfig("old", "standby_old", "us-east-1b", true)
	next := physicalMemberConfig("next", "standby_next", "us-east-1c", true)
	for _, tc := range []struct {
		name, members, key string
		retrofit           bool
	}{
		{name: "same address", members: physicalMemberConfig("old", "standby_next", "us-east-1c", true), key: "old"},
		{name: "different resource address", members: next, key: "next"},
		{name: "retrofit outgoing lifecycle", members: next, key: "next", retrofit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts, mock := newVirtualClusterMock(t)
			defer ts.Close()
			steps := []resource.TestStep{{Config: memberReplacementConfig(ts, physicalMemberConfig("old", "standby_old", "us-east-1b", !tc.retrofit), "old")}}
			if tc.retrofit {
				steps = append(steps, resource.TestStep{
					Config: memberReplacementConfig(ts, old, "old"),
					Check: func(*terraform.State) error {
						mock.mu.Lock()
						defer mock.mu.Unlock()
						if mock.clusterCreates != 1 || mock.updates != 0 || len(mock.events) != 1 {
							return fmt.Errorf("recording lifecycle must not mutate members: %v", mock.events)
						}
						return nil
					},
				})
			}
			config := memberReplacementConfig(ts, tc.members, tc.key)
			steps = append(steps, resource.TestStep{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
				virtualClusterCheck("production", "c-a", "c-2", "c-a", "c-2"),
				func(*terraform.State) error {
					mock.mu.Lock()
					defer mock.mu.Unlock()
					want := []string{"create:c-1", "create:c-2", "pair:c-2", "delete:c-1"}
					if mock.creates != 1 || mock.deletes != 0 || len(mock.clusters) != 1 || !slices.Equal(mock.events, want) {
						return fmt.Errorf("replacement must preserve virtual cluster and detach before deletion: %v", mock.events)
					}
					return nil
				},
			)}, resource.TestStep{Config: config, PlanOnly: true})
			resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts), Steps: steps})
		})
	}
}

func TestAccVirtualClusterMemberReplacementRecovery(t *testing.T) {
	ts, mock := newVirtualClusterMock(t)
	defer ts.Close()
	old := physicalMemberConfig("old", "standby_old", "us-east-1b", true)
	next := physicalMemberConfig("next", "standby_next", "us-east-1c", true)
	config := memberReplacementConfig(ts, next, "next")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(ts),
		Steps: []resource.TestStep{
			{Config: memberReplacementConfig(ts, physicalMemberConfig("old", "standby_old", "us-east-1b", false), "old")},
			// The rule on the new instance cannot protect the removed instance's prior state.
			{Config: config, ExpectError: regexp.MustCompile(`keep or restore its\s+Terraform declaration`)},
			{Config: config, ExpectError: regexp.MustCompile(`keep or restore its\s+Terraform declaration`)},
			{Config: memberReplacementConfig(ts, old+next, "next"), Check: resource.ComposeAggregateTestCheckFunc(
				virtualClusterCheck("production", "c-a", "c-2", "c-a", "c-2"),
				resource.TestCheckResourceAttr("velodb_cluster.old", "id", "c-1"),
				resource.TestCheckResourceAttr("velodb_cluster.next", "id", "c-2"),
			)},
			{Config: config, Check: virtualClusterCheck("production", "c-a", "c-2", "c-a", "c-2")},
			{Config: config, PlanOnly: true},
		},
	})
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if mock.clusterCreates != 2 || mock.creates != 1 || mock.updates != 1 || len(mock.clusters) != 0 {
		t.Fatalf("recovery must retain tracked IDs and destroy cleanly: %v", mock.events)
	}
}
