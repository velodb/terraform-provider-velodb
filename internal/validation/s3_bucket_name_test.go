package validation

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestS3BucketName(t *testing.T) {
	valid := []string{"abc", "123", "velodb-data", "example.com", "a.-b", strings.Repeat("a", 63), "data-111122223333-us-east-1-an"}
	invalid := []string{
		"", " ", "ab", strings.Repeat("a", 64), "-velodb-data", "velodb-data-",
		".velodb-data", "velodb-data.", "VeloDB-data", "velodb_data", "velodb data", " velodb-data", "velodb-data\n", "vélodb-data",
		"velodb..data", "192.168.0.1", "999.999.999.999", "xn--bucket", "sthree-bucket", "amzn-s3-demo-bucket",
		"bucket-s3alias", "bucket--ol-s3", "bucket.mrap", "bucket--x-s3", "bucket--table-s3",
	}
	for _, names := range []struct {
		values []string
		bad    bool
	}{{valid, false}, {invalid, true}} {
		for _, name := range names.values {
			t.Run(name, func(t *testing.T) {
				if err := CheckS3BucketName(name); (err != nil) != names.bad {
					t.Fatalf("CheckS3BucketName(%q) = %v, want error: %v", name, err, names.bad)
				}
				var resp validator.StringResponse
				S3BucketName{}.ValidateString(context.Background(), validator.StringRequest{
					Path: path.Root("bucket_name"), ConfigValue: types.StringValue(name),
				}, &resp)
				if resp.Diagnostics.HasError() != names.bad {
					t.Fatalf("unexpected schema validation diagnostics: %v", resp.Diagnostics)
				}
			})
		}
	}
	for _, value := range []types.String{types.StringNull(), types.StringUnknown()} {
		var resp validator.StringResponse
		S3BucketName{}.ValidateString(context.Background(), validator.StringRequest{ConfigValue: value}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("null/unknown must be deferred: %v", resp.Diagnostics)
		}
	}
}
