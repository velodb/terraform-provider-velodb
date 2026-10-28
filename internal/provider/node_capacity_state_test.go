package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestNodeCapacityOldStateUpgrade(t *testing.T) {
	ctx := context.Background()
	server := providerserver.NewProtocol6(New("test")())()
	for _, tc := range []struct{ name, raw string }{
		{"velodb_cluster", `{"id":"c-old","warehouse_id":"w-old","name":"analytics","cluster_type":"COMPUTE","compute_vcpu":16,"cache_gb":400,"node_count":2}`},
		{"velodb_warehouse", `{"id":"w-old","name":"warehouse","deployment_mode":"SaaS","cloud_provider":"aws","region":"us-east-1","initial_cluster":[{"zone":"us-east-1a","compute_vcpu":16,"cache_gb":400}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upgraded, err := server.UpgradeResourceState(ctx, &tfprotov6.UpgradeResourceStateRequest{TypeName: tc.name, Version: 0, RawState: &tfprotov6.RawState{JSON: []byte(tc.raw)}})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range upgraded.Diagnostics {
				if d.Severity == tfprotov6.DiagnosticSeverityError {
					t.Fatalf("state upgrade failed: %s %s", d.Summary, d.Detail)
				}
			}
			schemas, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
			if err != nil {
				t.Fatal(err)
			}
			value, err := upgraded.UpgradedState.Unmarshal(schemas.ResourceSchemas[tc.name].ValueType())
			if err != nil {
				t.Fatal(err)
			}
			var attrs map[string]tftypes.Value
			if err := value.As(&attrs); err != nil {
				t.Fatal(err)
			}
			var id string
			if err := attrs["id"].As(&id); err != nil {
				t.Fatal(err)
			}
			if id == "" {
				t.Fatal("state upgrade lost identity")
			}
			if tc.name == "velodb_cluster" && !attrs["node_vcpu"].IsNull() {
				t.Fatal("state upgrade must not infer node capacity")
			}
		})
	}
}

func TestNodeCapacityConfigValidation(t *testing.T) {
	ctx := context.Background()
	server := providerserver.NewProtocol6(New("test")())()
	for _, tc := range []struct {
		capacity string
		valid    bool
	}{
		{`"node_vcpu":8,"node_count":3,"node_cache":200`, true},
		{`"cache_type":0,"node_vcpu":8,"node_count":3,"node_cache":200`, true},
		{`"cache_type":1,"node_vcpu":8,"node_count":3`, true},
		{`"cache_type":1,"node_vcpu":8,"node_count":3,"node_cache":474`, false},
		{`"cache_type":2,"node_vcpu":8,"node_count":3,"node_cache":200`, false},
		{`"cache_type":1,"compute_vcpu":8,"cache_gb":200`, false},
		{`"node_vcpu":8,"node_count":3`, false},
		{`"node_vcpu":8,"node_count":3,"node_cache":200,"compute_vcpu":0`, false},
		{`"node_vcpu":8.5,"node_count":3,"node_cache":200`, false},
		{`"node_vcpu":8,"node_count":0,"node_cache":200`, false},
	} {
		resp, err := server.ValidateResourceConfig(ctx, &tfprotov6.ValidateResourceConfigRequest{TypeName: "velodb_cluster", Config: &tfprotov6.DynamicValue{JSON: []byte(`{"warehouse_id":"w","name":"analytics","cluster_type":"COMPUTE",` + tc.capacity + `}`)}})
		if err != nil {
			t.Fatal(err)
		}
		invalid := false
		for _, d := range resp.Diagnostics {
			if d.Severity == tfprotov6.DiagnosticSeverityError {
				invalid = true
			}
		}
		if invalid == tc.valid {
			raw, _ := json.Marshal(resp.Diagnostics)
			t.Fatalf("capacity=%s valid=%v diagnostics=%s", tc.capacity, tc.valid, raw)
		}
	}
}
