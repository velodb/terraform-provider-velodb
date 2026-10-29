package resource

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func knownInt(value types.Int64) bool { return !value.IsNull() && !value.IsUnknown() }
func nodeCapacityConfigured(cpu, count, cache types.Int64) bool {
	return !cpu.IsNull() || !count.IsNull() || !cache.IsNull()
}
func optionalNodeValue(value *int) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*value))
}

func localCache(cacheType types.Int64) bool {
	return knownInt(cacheType) && cacheType.ValueInt64() == 1
}

func validateCacheTypeChange(diags *diag.Diagnostics, configured, prior types.Int64) {
	if knownInt(configured) && !configured.Equal(prior) {
		diags.AddError("cache_type is creation-only", "Cache type cannot be changed on an existing cluster. If the API does not return cacheType, deploy the cache-type API before configuring this field.")
	}
}

func validateCapacity(diags *diag.Diagnostics, cpu, cache, nodeCPU, count, nodeCache, cacheType types.Int64) {
	// Module expressions can be unknown during initial configuration validation.
	for _, value := range []types.Int64{cpu, cache, nodeCPU, count, nodeCache, cacheType} {
		if value.IsUnknown() {
			return
		}
	}
	if knownInt(cacheType) && cacheType.ValueInt64() != 0 && cacheType.ValueInt64() != 1 {
		diags.AddError("Invalid cache_type", "cache_type must be 0 (EBS) or 1 (local NVMe).")
	}
	local := localCache(cacheType)
	if local && !nodeCache.IsNull() {
		diags.AddError("Local cache is package-defined", "Omit node_cache when cache_type is 1. Its applied value is read from the API and changes with the node package.")
	}
	if local || nodeCapacityConfigured(nodeCPU, count, nodeCache) {
		if !cpu.IsNull() || !cache.IsNull() {
			diags.AddError("Conflicting capacity inputs", "Use either compute_vcpu/cache_gb or node_vcpu/node_count/node_cache. Do not combine them, even with zero values.")
			return
		}
		if nodeCPU.IsNull() || count.IsNull() || !local && nodeCache.IsNull() {
			diags.AddError("Incomplete node capacity", "Set node_vcpu and node_count. EBS also requires node_cache in GB per node; omit node_cache for local NVMe.")
		}
		for name, value := range map[string]types.Int64{"node_vcpu": nodeCPU, "node_count": count, "node_cache": nodeCache} {
			if knownInt(value) && (value.ValueInt64() < 1 || value.ValueInt64() > math.MaxInt32) {
				diags.AddAttributeError(path.Root(name), "Invalid node capacity", name+" must be a positive integer no greater than 2147483647.")
			}
		}
		if knownInt(nodeCPU) && knownInt(count) && nodeCPU.ValueInt64() > 0 && count.ValueInt64() > math.MaxInt32/nodeCPU.ValueInt64() {
			diags.AddError("Node capacity exceeds API limits", "node_vcpu multiplied by node_count must fit a signed 32-bit integer.")
		}
	} else if cpu.IsNull() || cache.IsNull() {
		diags.AddError("Capacity inputs required", "Set compute_vcpu and cache_gb, or node_vcpu, node_count and node_cache.")
	}
}

func (r *ClusterResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || req.State.Raw.IsNull() {
		return
	}
	var config, plan, state ClusterResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateCreateOnlyRatio(&resp.Diagnostics, plan.Ratio, state.Ratio)
	validateCacheTypeChange(&resp.Diagnostics, config.CacheType, state.CacheType)
	if localCache(state.CacheType) && config.NodeCache.IsNull() && !plan.NodeVcpu.Equal(state.NodeVcpu) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("node_cache"), types.Int64Unknown())...)
	}
	if nodeCapacityConfigured(config.NodeVcpu, config.NodeCount, config.NodeCache) {
		if knownInt(plan.NodeCount) && knownInt(state.NodeCount) && !plan.NodeCount.Equal(state.NodeCount) && (knownInt(plan.NodeVcpu) && !plan.NodeVcpu.Equal(state.NodeVcpu) || knownInt(config.NodeCache) && !config.NodeCache.Equal(state.NodeCache)) {
			resp.Diagnostics.AddError("Separate node count and size changes", "Change node_count in one apply and node_vcpu/node_cache in another apply.")
		}

		// A model transition needs an explicit command even when geometry is unchanged.
		if state.CapacityModel.ValueString() != "NODE_SPEC" {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("capacity_model"), types.StringUnknown())...)
		}
	} else if state.CapacityModel.ValueString() == "NODE_SPEC" && ((!plan.ComputeVcpu.IsUnknown() && !plan.ComputeVcpu.Equal(state.ComputeVcpu)) || (!plan.CacheGb.IsUnknown() && !plan.CacheGb.Equal(state.CacheGb))) {
		resp.Diagnostics.AddError("Legacy capacity cannot resize a NODE_SPEC cluster", "Replace compute_vcpu/cache_gb with node_vcpu/node_count/node_cache on this same resource; no replacement is needed.")
	}
}

func (r *ClusterResource) updateNodeCapacity(ctx context.Context, warehouseID, clusterID string, config ClusterResourceModel, plan *ClusterResourceModel, state *ClusterResourceModel, timeout time.Duration, diags *diag.Diagnostics) bool {
	validateCapacity(diags, config.ComputeVcpu, config.CacheGb, config.NodeVcpu, config.NodeCount, config.NodeCache, config.CacheType)
	validateCreateOnlyRatio(diags, plan.Ratio, state.Ratio)
	if diags.HasError() {
		return false
	}
	cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
	if err != nil {
		diags.AddError(userError("reading cluster before node resize", err))
		return false
	}
	if cl.NodeVcpu == nil || cl.NodeCache == nil {
		diags.AddError("Node capacity API unavailable", "The deployed API must return nodeVcpu and nodeCache. Deploy the Formation node-capacity API before managing node settings.")
		return false
	}
	local := cl.CacheType != nil && *cl.CacheType == 1
	if knownInt(config.CacheType) && (cl.CacheType == nil || int64(*cl.CacheType) != config.CacheType.ValueInt64()) {
		diags.AddError("Cache type unavailable or changed", "Refresh the cluster and verify the deployed API returns the configured cacheType before resizing.")
		return false
	}
	if local && !config.NodeCache.IsNull() {
		diags.AddError("Local cache is package-defined", "Omit node_cache before updating a local NVMe cluster.")
		return false
	}
	countChanged := int64(cl.NodeCount) != plan.NodeCount.ValueInt64()
	cpuChanged := int64(*cl.NodeVcpu) != plan.NodeVcpu.ValueInt64()
	sizeChanged := cpuChanged || !local && int64(*cl.NodeCache) != plan.NodeCache.ValueInt64()
	promote := cl.CapacityModel != "NODE_SPEC"
	modeChanged := !config.ScalingMode.IsNull() && !config.ScalingMode.Equal(state.ScalingMode)
	if !countChanged && !sizeChanged && !promote && !modeChanged {
		return true
	}
	if cl.OnDemandNodeCount > 0 && cl.SubscriptionNodeCount > 0 || cl.BillingSummary != nil && cl.BillingSummary.IsMixedBilling {
		diags.AddError("Mixed-billing resize is not supported", "node_count is the whole-cluster count, while mixed-billing updates require a selected billing pool. Resize the pools outside this resource until explicit pool selection is supported.")
		return false
	}
	if countChanged && sizeChanged {
		diags.AddError("Separate node count and size changes", "Change node_count in one apply and node_vcpu/node_cache in another apply. Manager accepts these as separate capacity commands.")
		return false
	}
	if !local && plan.NodeCache.ValueInt64() < int64(*cl.NodeCache) && !cpuChanged {
		diags.AddError("node_cache cannot be decreased", "Cache-only shrinking is not supported. Review the applied package before changing node size.")
		return false
	}
	update := &client.UpdateClusterRequest{}
	if countChanged {
		setOptionalIntFromInt64(&update.NodeCount, plan.NodeCount)
	} else if sizeChanged && !cpuChanged && !promote {
		if !config.ScalingMode.IsNull() {
			diags.AddError("Cache-only updates cannot switch scaling mode", "Remove scaling_mode for the cache update, or switch to MANUAL separately with a node CPU/count operation.")
			return false
		}
		if !local {
			setOptionalIntFromInt64(&update.NodeCache, plan.NodeCache)
		}
	} else {
		setOptionalIntFromInt64(&update.NodeVcpu, plan.NodeVcpu)
		if !local {
			setOptionalIntFromInt64(&update.NodeCache, plan.NodeCache)
		}
	}
	if !config.ScalingMode.IsNull() && !config.ScalingMode.IsUnknown() {
		update.ScalingMode = config.ScalingMode.ValueString()
	}
	if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, update); err != nil {
		diags.AddError(userError("updating node capacity", err))
		return false
	}
	_, err = client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
		current, err := r.client.GetCluster(ctx, warehouseID, clusterID)
		if err != nil {
			return "", err
		}
		setClusterState(current, state, diags)
		return nodeCapacityStatus(current, *plan), nil
	}, client.StableStatuses, client.FailedStatuses, timeout, 15*time.Second)
	if err != nil {
		diags.AddError("Node capacity operation incomplete", fmt.Sprintf("The cluster ID is retained. Refresh before retrying; a submitted operation may still finish: %v", err))
		return false
	}
	state.ScalingMode = plan.ScalingMode
	return true
}

func nodeCapacityStatus(cl *client.ClusterDetail, plan ClusterResourceModel) string {
	for _, failed := range client.FailedStatuses {
		if cl.Status == failed {
			return cl.Status
		}
	}
	if knownInt(plan.CacheType) && (cl.CacheType == nil || int64(*cl.CacheType) != plan.CacheType.ValueInt64()) {
		return "CapacityPending"
	}
	if cl.CapacityModel != "NODE_SPEC" || cl.NodeVcpu == nil || cl.NodeCache == nil || int64(*cl.NodeVcpu) != plan.NodeVcpu.ValueInt64() || *cl.NodeCache <= 0 || knownInt(plan.NodeCache) && int64(*cl.NodeCache) != plan.NodeCache.ValueInt64() || int64(cl.NodeCount) != plan.NodeCount.ValueInt64() {
		return "CapacityPending"
	}
	return cl.Status
}

func validateInitialNodeCapacityChange(ctx context.Context, diags *diag.Diagnostics, plan, state types.List) {
	if plan.IsUnknown() || state.IsUnknown() || plan.IsNull() && state.IsNull() {
		return
	}
	var next, prior []InitialClusterModel
	if !plan.IsNull() {
		diags.Append(plan.ElementsAs(ctx, &next, false)...)
	}
	if !state.IsNull() {
		diags.Append(state.ElementsAs(ctx, &prior, false)...)
	}
	if diags.HasError() {
		return
	}
	var a, b InitialClusterModel
	if len(next) > 0 {
		a = next[0]
	}
	if len(prior) > 0 {
		b = prior[0]
	}
	if (nodeCapacityConfigured(a.NodeVcpu, a.NodeCount, a.NodeCache) || nodeCapacityConfigured(b.NodeVcpu, b.NodeCount, b.NodeCache) || !a.CacheType.Equal(b.CacheType)) && !plan.Equal(state) {
		diags.AddError("initial_cluster is creation-only", "Keep the initial_cluster creation inputs unchanged. Import its known ID into a velodb_cluster resource to manage later resizing; do not replace the warehouse.")
	}
}
