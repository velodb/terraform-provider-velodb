package resource

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func initialCapacity(ctx context.Context, list types.List, diags *diag.Diagnostics) *InitialClusterModel {
	if list.IsNull() || list.IsUnknown() || len(list.Elements()) == 0 {
		return nil
	}
	var blocks []InitialClusterModel
	diags.Append(list.ElementsAs(ctx, &blocks, false)...)
	if diags.HasError() {
		return nil
	}
	if len(blocks) != 1 {
		diags.AddError("Invalid initial_cluster", "Exactly one initial_cluster block is supported.")
		return nil
	}
	return &blocks[0]
}

func initialCapacityModel(block InitialClusterModel) ClusterResourceModel {
	return ClusterResourceModel{CapacityModel: block.CapacityModel, ComputeVcpu: block.ComputeVcpu, CacheGb: block.CacheGb, NodeVcpu: block.NodeVcpu, NodeCount: block.NodeCount, NodeCache: block.NodeCache, CacheStorageType: block.CacheStorageType, MemoryPerVcpu: block.MemoryPerVcpu, Ratio: block.Ratio}
}

// Compare explicit inputs only; local cache and omitted creation defaults are API-defined.
func initialCapacityMatches(ctx context.Context, config, actual types.List, diags *diag.Diagnostics) bool {
	wanted := initialCapacity(ctx, config, diags)
	observed := initialCapacity(ctx, actual, diags)
	if wanted == nil || observed == nil || diags.HasError() {
		return false
	}
	matched := true
	for _, field := range []struct {
		name, label, unit string
		wanted, actual    types.Int64
	}{
		{"compute_vcpu", "compute capacity", " vCPUs", wanted.ComputeVcpu, observed.ComputeVcpu},
		{"cache_gb", "cache", " GB", wanted.CacheGb, observed.CacheGb},
		{"node_vcpu", "vCPUs per node", "", wanted.NodeVcpu, observed.NodeVcpu},
		{"node_count", "node count", "", wanted.NodeCount, observed.NodeCount},
		{"node_cache", "cache per node", " GB", wanted.NodeCache, observed.NodeCache},
	} {
		if field.wanted.IsNull() || field.name == "node_cache" && localCache(wanted.CacheStorageType) {
			continue
		}
		if field.wanted.IsUnknown() {
			matched = false
			continue
		}
		if !knownInt(field.actual) || !field.wanted.Equal(field.actual) {
			detail := fmt.Sprintf("initial_cluster.%s is %s in the API, but configuration specifies %s. Match the current capacity and apply once before resizing. Remove initial_cluster instead if a separate velodb_cluster resource owns this cluster. This migration does not resize the cluster.", field.name, field.actual.String(), field.wanted.String())
			if knownInt(field.actual) {
				detail = fmt.Sprintf("Initial cluster %s is currently %s%s; configuration specifies %s%s. To prevent an unintended resize during migration, set %s = %s and apply once. You can resize afterward.", field.label, field.actual.String(), field.unit, field.wanted.String(), field.unit, field.name, field.actual.String())
			}
			diags.AddError("Initial cluster capacity adoption required", detail)
			matched = false
		}
	}
	return matched
}

func validateInitialCapacityPlan(ctx context.Context, diags *diag.Diagnostics, plan, state WarehouseResourceModel, config types.List) {
	next := initialCapacity(ctx, plan.InitialCluster, diags)
	prior := initialCapacity(ctx, state.InitialCluster, diags)
	configured := initialCapacity(ctx, config, diags)
	// Removing the block relinquishes management.
	if diags.HasError() || next == nil {
		return
	}
	if prior == nil {
		diags.AddError("Initial cluster is unmanaged", "Warehouse import does not adopt an initial cluster. Omit initial_cluster and import a verified physical cluster separately as velodb_cluster.")
		return
	}
	capacityChanged := !next.ComputeVcpu.Equal(prior.ComputeVcpu) || !next.CacheGb.Equal(prior.CacheGb) || !next.NodeVcpu.Equal(prior.NodeVcpu) || !next.NodeCount.Equal(prior.NodeCount) || configured != nil && !configured.NodeCache.IsNull() && !next.NodeCache.Equal(prior.NodeCache)
	if capacityChanged && (state.InitialClusterID.IsNull() || state.InitialClusterID.IsUnknown() || state.InitialClusterID.ValueString() == "") {
		diags.AddError("Initial cluster ID unavailable", "The creation-time initial_cluster_id is missing or its cluster was deleted. Restore the configuration; verify and import the physical cluster as velodb_cluster before resizing. The provider will not guess a target.")
		return
	}
	validateInitialClusterCreateOnlyRatio(ctx, diags, plan.InitialCluster, state.InitialCluster)
	if !next.Zone.Equal(prior.Zone) || !next.AutoPause.Equal(prior.AutoPause) {
		diags.AddError("Initial cluster creation settings cannot be changed", "Keep zone and auto_pause unchanged. This block manages capacity only after creation; use a separate velodb_cluster resource for other lifecycle settings.")
	}
	if configured == nil {
		return
	}
	priorCacheStorageType := prior.CacheStorageType
	if priorCacheStorageType.IsNull() {
		priorCacheStorageType = types.Int64Value(0)
	} // Omitted creation cache storage type defaults to EBS.
	validateCacheStorageTypeChange(diags, configured.CacheStorageType, priorCacheStorageType)
	validateLocalNodeCache(diags, prior.CacheStorageType, configured.NodeCache, next.NodeVcpu, prior.NodeVcpu)
	if diags.HasError() {
		return
	}
	if nodeCapacityConfigured(configured.NodeVcpu, configured.NodeCount, configured.NodeCache) {
		if knownInt(next.NodeCount) && knownInt(prior.NodeCount) && !next.NodeCount.Equal(prior.NodeCount) && (knownInt(next.NodeVcpu) && !next.NodeVcpu.Equal(prior.NodeVcpu) || knownInt(configured.NodeCache) && !configured.NodeCache.Equal(prior.NodeCache)) {
			diags.AddError("Separate node count and size changes", "Change initial_cluster.node_count in one apply and node_vcpu/node_cache in another apply.")
		}
	} else if knownInt(next.ComputeVcpu) && knownInt(next.CacheGb) && knownInt(prior.ComputeVcpu) && knownInt(prior.CacheGb) {
		legacyCapacityUpdates(initialCapacityModel(*next), initialCapacityModel(*prior), diags)
	}
}

func (r *WarehouseResource) initialClusterDetail(ctx context.Context, state WarehouseResourceModel) (*client.ClusterDetail, error) {
	cl, err := r.client.GetCluster(ctx, state.ID.ValueString(), state.InitialClusterID.ValueString())
	if err != nil {
		return nil, err
	}
	if cl.ClusterID != state.InitialClusterID.ValueString() || cl.WarehouseID != state.ID.ValueString() || cl.ClusterType != "COMPUTE" {
		return nil, errors.New("the saved initial_cluster_id must identify a compute cluster in this warehouse; no other cluster will be selected automatically")
	}
	return cl, nil
}

func saveInitialCapacity(ctx context.Context, state *WarehouseResourceModel, block InitialClusterModel, observed ClusterResourceModel, diags *diag.Diagnostics) {
	if !block.NodeVcpu.IsNull() || !block.NodeCount.IsNull() {
		if !knownInt(observed.NodeVcpu) || !knownInt(observed.NodeCache) || !knownInt(observed.NodeCount) || observed.NodeVcpu.ValueInt64() <= 0 || observed.NodeCache.ValueInt64() <= 0 || observed.NodeCount.ValueInt64() <= 0 {
			diags.AddError("Node capacity API unavailable", "The API must return applied nodeVcpu, nodeCount and nodeCache before initial-cluster capacity can be managed.")
			return
		}
		block.NodeVcpu, block.NodeCount = observed.NodeVcpu, observed.NodeCount
		block.NodeCache = observed.NodeCache
	} else {
		if !knownInt(observed.ComputeVcpu) || !knownInt(observed.CacheGb) || observed.ComputeVcpu.ValueInt64() <= 0 || observed.CacheGb.ValueInt64() <= 0 {
			diags.AddError("Initial cluster capacity unavailable", "The API did not return CPU/cache capacity. The requested values will not be saved as applied capacity.")
			return
		}
		block.ComputeVcpu, block.CacheGb = observed.ComputeVcpu, observed.CacheGb
		block.NodeCache = types.Int64Null()
	}
	if knownInt(observed.MemoryPerVcpu) || block.MemoryPerVcpu.IsUnknown() {
		block.MemoryPerVcpu = observed.MemoryPerVcpu
	}
	block.CapacityModel = observed.CapacityModel
	var d diag.Diagnostics
	state.InitialCluster, d = types.ListValueFrom(ctx, state.InitialCluster.ElementType(ctx), []InitialClusterModel{block})
	diags.Append(d...)
}

func (r *WarehouseResource) readInitialCapacity(ctx context.Context, state *WarehouseResourceModel, diags *diag.Diagnostics) {
	block := initialCapacity(ctx, state.InitialCluster, diags)
	if block == nil || state.ID.IsNull() || state.InitialClusterID.IsNull() || state.InitialClusterID.IsUnknown() || state.InitialClusterID.ValueString() == "" {
		return
	}
	cl, err := r.initialClusterDetail(ctx, *state)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			missingID := state.InitialClusterID.ValueString()
			state.InitialClusterID = types.StringNull()
			diags.AddWarning("Initial cluster no longer exists", fmt.Sprintf("Cluster %q was not found. The warehouse and last known capacity remain in state, but resizing is unavailable. Remove initial_cluster to relinquish management; the provider will not select or create another cluster.", missingID))
			return
		}
		diags.AddError(userError("reading initial cluster capacity", err))
		return
	}
	if cl.Status == "CreateFailed" {
		diags.AddError("Initial cluster provisioning failed", "The API reports CreateFailed for the saved initial cluster. The warehouse and cluster IDs are retained for recovery.")
		return
	}
	var observed ClusterResourceModel
	setClusterState(cl, &observed, diags)
	saveInitialCapacity(ctx, state, *block, observed, diags)
}

func (r *WarehouseResource) updateInitialCapacity(ctx context.Context, config types.List, plan WarehouseResourceModel, state *WarehouseResourceModel, timeout time.Duration, diags *diag.Diagnostics) {
	next := initialCapacity(ctx, plan.InitialCluster, diags)
	configured := initialCapacity(ctx, config, diags)
	if diags.HasError() || next == nil || configured == nil {
		return
	}
	// The spelling can change without a live cluster when the value is unchanged.
	if state.InitialClusterID.IsNull() || state.InitialClusterID.IsUnknown() || state.InitialClusterID.ValueString() == "" {
		prior := initialCapacity(ctx, state.InitialCluster, diags)
		if prior != nil && (!prior.MemoryPerVcpu.Equal(next.MemoryPerVcpu) || !prior.Ratio.Equal(next.Ratio)) {
			prior.MemoryPerVcpu, prior.Ratio = next.MemoryPerVcpu, next.Ratio
			var stateDiags diag.Diagnostics
			state.InitialCluster, stateDiags = types.ListValueFrom(ctx, state.InitialCluster.ElementType(ctx), []InitialClusterModel{*prior})
			diags.Append(stateDiags...)
		}
		return
	}
	cl, err := r.initialClusterDetail(ctx, *state)
	if err != nil {
		diags.AddError(userError("reading initial cluster capacity", err))
		return
	}
	if cl.Status == "CreateFailed" {
		diags.AddError("Initial cluster provisioning failed", "The API reports CreateFailed for the saved initial cluster. The warehouse and cluster IDs are retained for recovery.")
		return
	}
	var observed ClusterResourceModel
	setClusterState(cl, &observed, diags)
	nodeManaged := nodeCapacityConfigured(configured.NodeVcpu, configured.NodeCount, configured.NodeCache)
	target := initialCapacityModel(*next)
	// Older APIs omit ratio. In that case retain the value already checked by planning.
	if observed.MemoryPerVcpu.IsNull() && knownInt(target.MemoryPerVcpu) {
		observed.MemoryPerVcpu = target.MemoryPerVcpu
	}
	observed.Ratio = target.Ratio
	saveInitialCapacity(ctx, state, *configured, observed, diags)
	if diags.HasError() {
		return
	}

	if !state.InitialCapacityAdopted.ValueBool() {
		// Recheck at apply to protect saved plans and plans made with refresh disabled.
		if initialCapacityMatches(ctx, config, state.InitialCluster, diags) {
			state.InitialCapacityAdopted = types.BoolValue(true)
		} else if !diags.HasError() {
			diags.AddError("Initial cluster capacity adoption required", "Capacity inputs must be known and match the current cluster before resizing.")
		}
		return
	}

	resize := &ClusterResource{client: r.client}
	if nodeManaged {
		resize.updateNodeCapacity(ctx, state.ID.ValueString(), state.InitialClusterID.ValueString(), initialCapacityModel(*configured), &target, &observed, timeout, diags)
	} else {
		if cl.CapacityModel == "NODE_SPEC" && (!target.ComputeVcpu.Equal(observed.ComputeVcpu) || !target.CacheGb.Equal(observed.CacheGb)) {
			diags.AddError("Legacy capacity cannot resize a NODE_SPEC cluster", "Replace initial_cluster.compute_vcpu/cache_gb with node_vcpu/node_count/node_cache on this same warehouse; no replacement is needed.")
			return
		}
		updates := legacyCapacityUpdates(target, observed, diags)
		if !diags.HasError() {
			resize.applyLegacyCapacity(ctx, state.ID.ValueString(), state.InitialClusterID.ValueString(), updates, target, &observed, timeout, diags)
		}
	}
	// Record observations even when an accepted operation later fails.
	var observedDiags diag.Diagnostics
	saveInitialCapacity(ctx, state, *next, observed, &observedDiags)
	diags.Append(observedDiags...)
}
