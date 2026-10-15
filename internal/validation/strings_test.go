package validation

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestStringValidators(t *testing.T) {
	tests := []struct {
		name string
		v    validator.String
		good []string
		bad  []string
	}{
		{"trimmed nonempty", TrimmedNonEmpty{}, []string{"production", "Production data", "数据"}, []string{"", " ", "\t\n", " production", "production ", "production\n", "\u00a0production", "production\u2003"}},
		{"password", AdminPassword{}, []string{"Abcdef12", "Abcdefg!", "ABCDEF1!", "abcdef1!", "Ab1" + strings.Repeat("x", 17)}, []string{"", "Abc123!", "Ab1" + strings.Repeat("x", 18), "abcdefgh", "Abcdefgh", "abcdef12", "ABCDEF12", "1234567!", "Abcdef1 ", "Abcdef1\n", "Abcdef1/", "Abcdef1=", "Abcdef1é"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := []types.String{types.StringNull(), types.StringUnknown()}
			for _, s := range append(tt.good, tt.bad...) {
				values = append(values, types.StringValue(s))
			}
			for i, value := range values {
				var resp validator.StringResponse
				tt.v.ValidateString(context.Background(), validator.StringRequest{Path: path.Root("value"), ConfigValue: value}, &resp)
				if resp.Diagnostics.HasError() != (i >= len(tt.good)+2) {
					t.Errorf("case %d: unexpected diagnostics: %v", i, resp.Diagnostics)
				}
				if tt.name == "password" && len(value.ValueString()) >= 8 && strings.Contains(fmt.Sprint(resp.Diagnostics), value.ValueString()) {
					t.Errorf("case %d: password leaked in diagnostics", i)
				}
			}
		})
	}
	for _, ch := range `!@#$%^&*()_+{}[]:;<>,.?~\-` {
		var resp validator.StringResponse
		AdminPassword{}.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue("abcdef1" + string(ch))}, &resp)
		if resp.Diagnostics.HasError() {
			t.Errorf("supported special character %q rejected", ch)
		}
	}
}
