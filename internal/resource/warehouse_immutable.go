package resource

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// Unknown values are rejected too: a replacement dependency produces an unknown
// ID during planning, before Terraform has destroyed the existing dependency.
func rejectWarehouseInfrastructureChange(diags *diag.Diagnostics, p path.Path, planned, prior attr.Value) {
	if !planned.Equal(prior) {
		diags.AddAttributeError(p, "Warehouse infrastructure cannot be changed", fmt.Sprintf("%s is already used by this warehouse and cannot be changed after creation. Restore the original configuration. Automatic warehouse replacement is not supported; provision a separate warehouse for a migration.", p.String()))
	}
}

type warehouseImmutableString struct{}

func (warehouseImmutableString) Description(context.Context) string {
	return "Reject changes to infrastructure already used by a warehouse."
}
func (m warehouseImmutableString) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
func (warehouseImmutableString) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	// Some create-only strings are absent after import and cannot be read back.
	if req.StateValue.IsNull() {
		return
	}
	rejectWarehouseInfrastructureChange(&resp.Diagnostics, req.Path, req.PlanValue, req.StateValue)
}

type warehouseImmutableInt64 struct{ encryption bool }

func (warehouseImmutableInt64) Description(context.Context) string {
	return "Reject changes to infrastructure already used by a warehouse."
}
func (m warehouseImmutableInt64) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
func (m warehouseImmutableInt64) PlanModifyInt64(_ context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	if req.ConfigValue.IsNull() {
		// Optional+computed: omission retains the existing association.
		// Module input guards separately reject encryption flag changes.
		resp.PlanValue = req.StateValue
		return
	}
	if !m.encryption && req.StateValue.IsNull() {
		// Older backends may omit credential and network IDs after import.
		return
	}
	rejectWarehouseInfrastructureChange(&resp.Diagnostics, req.Path, req.PlanValue, req.StateValue)
}

// PlanModifyBool rejects changes to create-only settings without replacing the warehouse.
type warehouseImmutableBool struct{}

func (warehouseImmutableBool) Description(context.Context) string {
	return "Reject changes to settings already used by a warehouse."
}
func (m warehouseImmutableBool) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
func (warehouseImmutableBool) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	// This setting cannot be recovered after import; permit initial binding.
	if req.StateValue.IsNull() {
		return
	}
	rejectWarehouseInfrastructureChange(&resp.Diagnostics, req.Path, req.PlanValue, req.StateValue)
}
