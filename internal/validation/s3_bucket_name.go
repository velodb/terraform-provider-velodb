package validation

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var (
	s3BucketNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
	s3BucketIPPattern   = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,3}){3}$`)
)

// CheckS3BucketName validates general purpose bucket names, including existing buckets.
// Namespace, availability, ownership, and access require AWS checks.
func CheckS3BucketName(name string) error {
	if !s3BucketNamePattern.MatchString(name) {
		return fmt.Errorf("bucket_name must be 3–63 characters, contain only lowercase letters, numbers, periods, or hyphens, and start and end with a letter or number")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("bucket_name must not contain adjacent periods")
	}
	if s3BucketIPPattern.MatchString(name) {
		return fmt.Errorf("bucket_name must not be formatted as an IP address")
	}
	for _, prefix := range []string{"xn--", "sthree-", "amzn-s3-demo-"} {
		if strings.HasPrefix(name, prefix) {
			return fmt.Errorf("bucket_name must not start with reserved prefix %q", prefix)
		}
	}
	for _, suffix := range []string{"-s3alias", "--ol-s3", ".mrap", "--x-s3", "--table-s3"} {
		if strings.HasSuffix(name, suffix) {
			return fmt.Errorf("bucket_name must not end with reserved suffix %q", suffix)
		}
	}
	return nil
}

type S3BucketName struct{}

func (S3BucketName) Description(context.Context) string {
	return "must be a valid general purpose S3 bucket name"
}

func (v S3BucketName) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (S3BucketName) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if err := CheckS3BucketName(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid S3 bucket name", err.Error())
	}
}
