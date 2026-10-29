package client

import (
	"encoding/json"
	"testing"
)

func testInt(value int) *int { return &value }

func TestNodeCapacityJSON(t *testing.T) {
	for name, input := range map[string]any{
		"cluster":           CreateClusterRequest{NodeVcpu: testInt(8), NodeCount: testInt(3), NodeCache: testInt(200)},
		"warehouse initial": InitialClusterRequest{NodeVcpu: testInt(8), NodeCount: testInt(3), NodeCache: testInt(200)},
		"update":            UpdateClusterRequest{NodeCount: testInt(0)},
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			for _, legacy := range []string{"computeVcpu", "cacheGb"} {
				if _, ok := fields[legacy]; ok {
					t.Fatalf("legacy field leaked: %s", raw)
				}
			}
			expected := float64(3)
			if name == "update" {
				expected = 0
			}
			if fields["nodeCount"] != expected {
				t.Fatalf("count not preserved: %s", raw)
			}
		})
	}
	for _, raw := range []string{`{"nodeVcpu":8,"nodeCache":200,"capacityModel":"NODE_SPEC"}`, `{}`} {
		var detail ClusterDetail
		if err := json.Unmarshal([]byte(raw), &detail); err != nil {
			t.Fatal(err)
		}
		if raw == `{}` && (detail.NodeVcpu != nil || detail.NodeCache != nil || detail.CapacityModel != "") {
			t.Fatal("missing geometry was fabricated")
		}
	}
}

func TestCacheTypeJSON(t *testing.T) {
	for _, value := range []int{0, 1} {
		for _, input := range []any{CreateClusterRequest{CacheType: &value}, InitialClusterRequest{CacheType: &value}} {
			raw, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			if fields["cacheType"] != float64(value) {
				t.Fatalf("explicit cache type lost: %s", raw)
			}
		}
	}
	for _, raw := range []string{`{}`, `{"cacheType":null}`, `{"cacheType":0}`, `{"cacheType":1}`} {
		var cl ClusterDetail
		if err := json.Unmarshal([]byte(raw), &cl); err != nil {
			t.Fatal(err)
		}
		var expected map[string]*int
		if err := json.Unmarshal([]byte(raw), &expected); err != nil {
			t.Fatal(err)
		}
		if v := expected["cacheType"]; (v == nil) != (cl.CacheType == nil) || v != nil && *v != *cl.CacheType {
			t.Fatalf("cache type readback: %s", raw)
		}
	}
}
