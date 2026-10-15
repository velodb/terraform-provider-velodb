package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// Exercise Terraform's offline validation RPCs without configuring an API client.
func TestBucketNameConfigValidation(t *testing.T) {
	ctx := context.Background()
	server := providerserver.NewProtocol6(New("test")())()
	for _, name := range []string{"velodb_byoc_credential", "velodb_aws_data_access_policy", "velodb_aws_crossaccount_policy"} {
		for _, bucket := range []string{"-invalid", "valid-bucket"} {
			t.Run(name+"/"+bucket, func(t *testing.T) {
				attrs := `"role_arn":"arn:aws:iam::111122223333:role/data"`
				switch name {
				case "velodb_byoc_credential":
					attrs = `"cloud_provider":"aws", "name":"test", "region":"us-east-1",
					"data_credential_arn":"arn:aws:iam::111122223333:instance-profile/data",
					"deployment_credential_arn":"arn:aws:iam::111122223333:role/deploy"`
				case "velodb_aws_crossaccount_policy":
					attrs = `"data_credential_arn":"arn:aws:iam::111122223333:instance-profile/data"`
				}
				config := &tfprotov6.DynamicValue{JSON: []byte(fmt.Sprintf(`{"bucket_name":%q,%s}`, bucket, attrs))}
				var diags []*tfprotov6.Diagnostic
				if name == "velodb_byoc_credential" {
					resp, err := server.ValidateResourceConfig(ctx, &tfprotov6.ValidateResourceConfigRequest{TypeName: name, Config: config})
					if err != nil {
						t.Fatal(err)
					}
					diags = resp.Diagnostics
				} else {
					resp, err := server.ValidateDataResourceConfig(ctx, &tfprotov6.ValidateDataResourceConfigRequest{TypeName: name, Config: config})
					if err != nil {
						t.Fatal(err)
					}
					diags = resp.Diagnostics
				}
				found := false
				for _, d := range diags {
					if d.Summary == "Invalid S3 bucket name" && d.Severity == tfprotov6.DiagnosticSeverityError {
						found = true
					} else if d.Severity == tfprotov6.DiagnosticSeverityError {
						t.Errorf("unexpected error: %s: %s", d.Summary, d.Detail)
					}
				}
				if found != (bucket == "-invalid") {
					t.Fatalf("expected invalid bucket diagnostic: %v, got: %v", bucket == "-invalid", diags)
				}
			})
		}
	}
}
