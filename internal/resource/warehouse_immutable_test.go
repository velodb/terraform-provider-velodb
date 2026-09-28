package resource

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestWarehouseImmutableModifiers(t *testing.T) {
	ctx := context.Background()
	present := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})
	absent := tftypes.NewValue(present.Type(), nil)
	for _, tc := range []struct {
		name        string
		state, plan tftypes.Value
		prior, next types.String
		wantError   bool
	}{
		{"create", absent, present, types.StringNull(), types.StringValue("us-east-1"), false},
		{"destroy", present, absent, types.StringValue("us-east-1"), types.StringNull(), false},
		{"unchanged", present, present, types.StringValue("us-east-1"), types.StringValue("us-east-1"), false},
		{"region edit", present, present, types.StringValue("us-east-1"), types.StringValue("us-west-2"), true},
		{"unknown dependency", present, present, types.StringValue("old"), types.StringUnknown(), true},
		{"removed", present, present, types.StringValue("old"), types.StringNull(), true},
		{"import initialization", present, present, types.StringNull(), types.StringValue("advanced"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.StringRequest{Path: path.Root("region"), State: tfsdk.State{Raw: tc.state}, Plan: tfsdk.Plan{Raw: tc.plan}, StateValue: tc.prior, PlanValue: tc.next}
			resp := planmodifier.StringResponse{PlanValue: tc.next}
			(warehouseImmutableString{}).PlanModifyString(ctx, req, &resp)
			if resp.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("diagnostics: %v", resp.Diagnostics)
			}
			if resp.RequiresReplace {
				t.Fatal("must not schedule replacement")
			}
		})
	}
	for _, tc := range []struct {
		name                string
		encryption          bool
		prior, config, next types.Int64
		wantError           bool
	}{
		{"credential changed", false, types.Int64Value(1), types.Int64Value(2), types.Int64Value(2), true},
		{"network replacement", false, types.Int64Value(1), types.Int64Unknown(), types.Int64Unknown(), true},
		{"import credential", false, types.Int64Null(), types.Int64Value(1), types.Int64Value(1), false},
		{"enable encryption", true, types.Int64Null(), types.Int64Value(2), types.Int64Value(2), true},
		{"replace encryption", true, types.Int64Value(1), types.Int64Unknown(), types.Int64Unknown(), true},
		{"unchanged encryption", true, types.Int64Value(1), types.Int64Value(1), types.Int64Value(1), false},
		{"unmanaged encryption", true, types.Int64Value(1), types.Int64Null(), types.Int64Unknown(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.Int64Request{Path: path.Root("key_id"), State: tfsdk.State{Raw: present}, Plan: tfsdk.Plan{Raw: present}, StateValue: tc.prior, ConfigValue: tc.config, PlanValue: tc.next}
			resp := planmodifier.Int64Response{PlanValue: tc.next}
			(warehouseImmutableInt64{encryption: tc.encryption}).PlanModifyInt64(ctx, req, &resp)
			if resp.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("diagnostics: %v", resp.Diagnostics)
			}
			if resp.RequiresReplace {
				t.Fatal("must not schedule replacement")
			}
			if tc.encryption && tc.config.IsNull() && !resp.PlanValue.Equal(tc.prior) {
				t.Fatal("omitted computed key must retain state")
			}
		})
	}
}
