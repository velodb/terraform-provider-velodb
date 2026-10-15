package validation

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// TrimmedNonEmpty preserves the configured value when the API trims whitespace.
type TrimmedNonEmpty struct{}

func (TrimmedNonEmpty) Description(context.Context) string {
	return "must not be empty or contain leading or trailing whitespace"
}

func (v TrimmedNonEmpty) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v TrimmedNonEmpty) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	if value == "" || value != strings.TrimSpace(value) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid value", v.Description(ctx))
	}
}

type AdminPassword struct{}

func (AdminPassword) Description(context.Context) string {
	return "must be 8–20 characters using only letters, digits, and !@#$%^&*()_+{}[]:;<>,.?~\\-; include at least three of uppercase letters, lowercase letters, digits, and special characters"
}

func (v AdminPassword) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v AdminPassword) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	password := req.ConfigValue.ValueString()
	classes := [4]bool{}
	valid := len(password) >= 8 && len(password) <= 20
	for _, ch := range password {
		switch {
		case ch >= 'A' && ch <= 'Z':
			classes[0] = true
		case ch >= 'a' && ch <= 'z':
			classes[1] = true
		case ch >= '0' && ch <= '9':
			classes[2] = true
		case strings.ContainsRune(`!@#$%^&*()_+{}[]:;<>,.?~\-`, ch):
			classes[3] = true
		default:
			valid = false
		}
	}
	count := 0
	for _, present := range classes {
		if present {
			count++
		}
	}
	if !valid || count < 3 {
		// Never include the sensitive configuration value in diagnostics.
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid administrator password", v.Description(ctx))
	}
}
