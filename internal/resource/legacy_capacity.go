package resource

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func legacyCapacityUpdates(plan, state ClusterResourceModel, diags *diag.Diagnostics) []client.UpdateClusterRequest {
	cpuChanged := !plan.ComputeVcpu.Equal(state.ComputeVcpu)
	diskChanged := !plan.CacheGb.Equal(state.CacheGb)
	validateCreateOnlyRatio(diags, plan.Ratio, state.Ratio)
	if diags.HasError() {
		return nil
	}
	cacheChangeHandledByCPU := false
	apiCacheAfterCPUResize := int64(0)
	hasAPIResizeCache := cpuChanged &&
		!plan.ComputeVcpu.IsUnknown() && !state.ComputeVcpu.IsUnknown() &&
		!plan.CacheGb.IsUnknown() && !state.CacheGb.IsUnknown()
	if hasAPIResizeCache {
		apiCacheAfterCPUResize = cacheGbAfterCPUResize(
			state.ComputeVcpu.ValueInt64(),
			state.CacheGb.ValueInt64(),
			plan.ComputeVcpu.ValueInt64(),
		)
	}
	if cpuChanged && diskChanged && hasAPIResizeCache && plan.CacheGb.ValueInt64() == apiCacheAfterCPUResize {
		cacheChangeHandledByCPU = true
	} else if diskChanged && !plan.CacheGb.IsUnknown() && !state.CacheGb.IsUnknown() &&
		plan.CacheGb.ValueInt64() < state.CacheGb.ValueInt64() {
		diags.AddError(
			"cache_gb cannot be decreased",
			"VeloDB does not support shrinking cluster cache_gb. "+
				"Increase cache_gb only, or recreate the cluster if you need a smaller disk.",
		)
		return nil
	} else if cpuChanged && diskChanged {
		diags.AddError(
			"Simultaneous compute_vcpu and cache_gb changes are not supported",
			fmt.Sprintf(
				"VeloDB automatically adjusts cache_gb to %d when resizing compute_vcpu from %d to %d. "+
					"Set cache_gb to %d for the CPU resize, then apply any additional cache_gb change in a later apply.",
				apiCacheAfterCPUResize,
				state.ComputeVcpu.ValueInt64(),
				plan.ComputeVcpu.ValueInt64(),
				apiCacheAfterCPUResize,
			),
		)
		return nil
	}
	if cpuChanged && !diskChanged && hasAPIResizeCache && apiCacheAfterCPUResize != plan.CacheGb.ValueInt64() {
		diags.AddError(
			"compute_vcpu resize requires cache_gb update",
			fmt.Sprintf(
				"VeloDB automatically adjusts cache_gb to %d when resizing compute_vcpu from %d to %d. "+
					"Set cache_gb to %d and apply again. Keeping cache_gb at %d requires a later cache-only apply after the API cooldown.",
				apiCacheAfterCPUResize,
				state.ComputeVcpu.ValueInt64(),
				plan.ComputeVcpu.ValueInt64(),
				apiCacheAfterCPUResize,
				plan.CacheGb.ValueInt64(),
			),
		)
		return nil
	}

	var updates []client.UpdateClusterRequest
	if cpuChanged {
		v := int(plan.ComputeVcpu.ValueInt64())
		updates = append(updates, client.UpdateClusterRequest{ComputeVcpu: &v})
	}
	if !cacheChangeHandledByCPU && diskChanged {
		v := int(plan.CacheGb.ValueInt64())
		updates = append(updates, client.UpdateClusterRequest{CacheGb: &v})
	}
	return updates
}

func (r *ClusterResource) applyLegacyCapacity(ctx context.Context, warehouseID, clusterID string, updates []client.UpdateClusterRequest, plan ClusterResourceModel, state *ClusterResourceModel, timeout time.Duration, diags *diag.Diagnostics) bool {
	for _, update := range updates {
		if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, &update); err != nil {
			diags.AddError(userError("resizing cluster capacity", err))
			return false
		}
		_, err := client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
			cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
			if err != nil {
				return "", err
			}
			setClusterState(cl, state, diags)
			if !state.ComputeVcpu.Equal(plan.ComputeVcpu) || !state.CacheGb.Equal(plan.CacheGb) {
				for _, failed := range client.FailedStatuses {
					if cl.Status == failed {
						return cl.Status, nil
					}
				}
				return "CapacityPending", nil
			}
			return cl.Status, nil
		}, client.StableStatuses, client.FailedStatuses, timeout, 15*time.Second)
		if err != nil {
			diags.AddError("Cluster capacity operation incomplete", fmt.Sprintf("The cluster ID is retained. Refresh before retrying; a submitted operation may still finish: %v", err))
			return false
		}
	}
	return true
}
