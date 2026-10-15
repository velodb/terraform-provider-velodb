package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestResourceInputValidation(t *testing.T) {
	server := providerserver.NewProtocol6(New("test")())()
	tests := []struct {
		resource string
		config   map[string]any
		fields   []string
	}{
		{"velodb_byoc_credential", map[string]any{
			"cloud_provider": "aws", "name": "production data", "region": "us-east-1", "bucket_name": "velodb-data",
			"data_credential_arn": "arn:aws:iam::111122223333:instance-profile/data", "deployment_credential_arn": "arn:aws:iam::111122223333:role/deploy",
		}, []string{"name", "region"}},
		{"velodb_byoc_network", map[string]any{
			"cloud_provider": "aws", "name": "production network", "credential_id": 1, "security_group_id": "sg-test",
			"zone_mappings": []map[string]any{{"zone_id": "us-east-1a", "subnet_id": "subnet-test"}},
		}, []string{"name"}},
		{"velodb_encryption_key", map[string]any{
			"cloud_provider": "aws", "name": "production key", "use_tde": true, "key_arn": "arn:aws:kms:us-east-1:111122223333:key/test-key",
		}, []string{"name"}},
		{"velodb_private_link_endpoint_service", map[string]any{
			"cloud_provider": "aws", "region": "us-east-1", "endpoint_service_id": "vpce-svc-test", "endpoint_service_name": "com.amazonaws.vpce.us-east-1.vpce-svc-test",
		}, []string{"cloud_provider", "region", "endpoint_service_id", "endpoint_service_name"}},
		{"velodb_warehouse_private_endpoint", map[string]any{
			"warehouse_id": "w-test", "endpoint_id": "vpce-test",
		}, []string{"warehouse_id", "endpoint_id"}},
		{"velodb_warehouse", map[string]any{
			"name": "production", "deployment_mode": "SaaS", "cloud_provider": "aws", "region": "us-east-1", "admin_password": "TestPass@123",
		}, []string{"admin_password"}},
	}
	for _, tt := range tests {
		for _, field := range tt.fields {
			original := tt.config[field]
			values := []any{"", " \t", " padded", "padded ", original}
			if field == "admin_password" {
				values = []any{"short", "Abcdefgh", "Abcdef1/", "Abcdef12", original, nil}
			}
			for i, value := range values {
				t.Run(fmt.Sprintf("%s/%s/%d", tt.resource, field, i), func(t *testing.T) {
					tt.config[field] = value
					data, err := json.Marshal(tt.config)
					if err != nil {
						t.Fatal(err)
					}
					resp, err := server.ValidateResourceConfig(context.Background(), &tfprotov6.ValidateResourceConfigRequest{
						TypeName: tt.resource, Config: &tfprotov6.DynamicValue{JSON: data},
					})
					if err != nil {
						t.Fatal(err)
					}
					invalid := i < 4
					if field == "admin_password" {
						invalid = i < 3
					}
					found := false
					for _, d := range resp.Diagnostics {
						if d.Severity == tfprotov6.DiagnosticSeverityError {
							lengthError := field == "name" && value == "" && d.Summary == "Invalid Attribute Value Length"
							if d.Summary != "Invalid value" && d.Summary != "Invalid administrator password" && !lengthError {
								t.Errorf("unexpected validation error: %s: %s", d.Summary, d.Detail)
							}
							found = true
						}
					}
					if found != invalid {
						t.Fatalf("expected error=%v, got %v", invalid, resp.Diagnostics)
					}
				})
			}
			tt.config[field] = original
		}
	}
}
