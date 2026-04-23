package resource

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// cacheGbAutoScaleOnVcpuChange marks cache_gb as unknown when the sibling
// compute_vcpu in the same pool block changes. This reflects the API behavior
// that CPU scaling auto-scales disk proportionally.
type cacheGbAutoScaleOnVcpuChange struct{}

func (m cacheGbAutoScaleOnVcpuChange) Description(_ context.Context) string {
	return "Marks cache_gb as known-after-apply when compute_vcpu changes (API auto-scales disk)."
}

func (m cacheGbAutoScaleOnVcpuChange) MarkdownDescription(_ context.Context) string {
	return m.Description(context.Background())
}

func (m cacheGbAutoScaleOnVcpuChange) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	// Only relevant when both prior state and config exist (i.e., updates, not create)
	if req.State.Raw.IsNull() {
		return
	}

	// Find sibling compute_vcpu in same block
	// req.Path is like subscription[0].cache_gb → sibling is subscription[0].compute_vcpu
	siblingPath := req.Path.ParentPath().AtName("compute_vcpu")

	var stateVcpu, planVcpu types.Int64
	diags := req.State.GetAttribute(ctx, siblingPath, &stateVcpu)
	resp.Diagnostics.Append(diags...)
	diags = req.Plan.GetAttribute(ctx, siblingPath, &planVcpu)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Mark cache_gb unknown when compute_vcpu changes — API auto-scales disk proportionally
	// in both directions. Only trigger when user didn't explicitly change cache_gb.
	if !stateVcpu.IsNull() && !planVcpu.IsNull() && !stateVcpu.Equal(planVcpu) {
		var stateCache, configCache types.Int64
		req.State.GetAttribute(ctx, req.Path, &stateCache)
		req.Config.GetAttribute(ctx, req.Path, &configCache)
		if !stateCache.IsNull() && !configCache.IsNull() && stateCache.Equal(configCache) {
			resp.PlanValue = types.Int64Unknown()
		}
	}
	_ = path.Empty() // silence unused import
}
