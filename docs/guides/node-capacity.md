# Node-based cluster capacity

Requires a provider build with node-capacity support and the corresponding Formation API deployment. Older API versions do not return `nodeVcpu`/`nodeCache`/`cacheType`; the provider reports this when a node-managed update cannot be verified.

A node is one compute machine. `node_vcpu` and `node_cache` describe each machine; `node_count` is the whole-cluster machine count. For example, three nodes with 8 vCPUs and 200 GB cache each have 24 total vCPUs and 600 GB total cache.

## Create

Use exactly one capacity input family:

- Legacy: `compute_vcpu` and `cache_gb`.
- EBS nodes: `node_vcpu`, `node_count`, and `node_cache`; optionally set `cache_type = 0`.
- Local NVMe nodes: `cache_type = 1`, `node_vcpu`, and `node_count`; omit `node_cache`.

EBS is the creation default when cache_type is omitted. Capacity values must be positive integers; package sizing, cache steps and stock remain server-validated. For local NVMe, node_cache is computed from the selected package and cannot be configured. The API ignores supplied local cache sizes; the provider rejects them to prevent an ignored configuration from disagreeing with Terraform state. Cache type is creation-only; changing it is rejected without proposing replacement.

```terraform
resource "velodb_cluster" "analytics" {
  warehouse_id = var.warehouse_id
  name         = "analytics"
  cluster_type = "COMPUTE"
  node_vcpu    = 8
  node_count   = 3
  node_cache   = 200
  ratio        = 8 # Creation only; omitted by every update request.
}
```

`capacity_model`, `total_cpu`, and `total_disk_gb` are observations. The legacy and node capacity attributes also return API observations when they are not configured; those observations do not select a management mode. Configuration determines which input family the provider sends. An absent capacity model remains null, not an inferred `NODE_SPEC` value.

For local NVMe:

```terraform
resource "velodb_cluster" "local" {
  warehouse_id = var.warehouse_id
  name         = "local_worker"
  cluster_type = "COMPUTE"
  cache_type   = 1
  ratio        = 4
  node_vcpu    = 4
  node_count   = 2
  # node_cache is read from the API, not configured.
}
```

## Update

Change `node_vcpu` and, when required by the selected package, `node_cache` together. Change `node_count` in a separate apply. The provider rejects a single apply that changes both count and node size/cache. Cache-only shrinking is unsupported. For local NVMe, changing node_vcpu selects a new package and the provider reads the resulting node_cache; no cache size or cache_type is sent in update requests. Node count can be changed independently. Resizing preserves the existing memory-to-vCPU ratio.

The provider waits for both a stable status and the requested geometry, so an old `Running` response cannot prematurely complete a resize. On timeout, refresh before retrying: an accepted operation can still finish. Backend cooldowns can require waiting before the next separate apply.

To explicitly leave Auto/Scheduled scaling during a node CPU or count command, set `scaling_mode = "MANUAL"`. This is an instruction, not a readback of the current mode. A cache-only update cannot switch mode; remove this instruction for that apply or perform a supported mode change first. The provider does not silently disable automatic scaling. Creating and editing autoscaling policies is outside this feature.

First-release node management supports a single billing pool. Mixed-billing clusters can be read, but a capacity change requires pool selection that this resource does not expose, so ambiguous updates are rejected. `node_count = 0` is not a supported whole-cluster target; removing an individual billing pool is not exposed.

## Migrate an existing cluster without replacement

Keep the same resource address and remote ID. Replace the legacy capacity inputs with the applied per-node values returned by the API:

```terraform
# Before:
# compute_vcpu = 24
# cache_gb     = 600

# After, on the SAME velodb_cluster resource:
# node_vcpu  = 8
# node_count = 3
# node_cache = 200
```

Do not change `ratio` during migration. Review `terraform plan`: migration must be an in-place update, never a replacement. Even when the physical size is unchanged, the provider submits a node command to promote an unmarked model. Manager controls that transaction and the one-way transition to `NODE_SPEC`. After promotion, legacy CPU/cache resizing is rejected; unrelated supported changes remain possible.

Import uses the existing `warehouse_id/cluster_id` format. Readback populates recorded model, node geometry and totals; configure the matching family and verify a no-change plan.

## Warehouse and module inputs

`velodb_warehouse.initial_cluster` accepts the same two creation families. Its block is creation-only and is retained as creation input because the warehouse GET does not return those settings. Warehouse import does not reconstruct or select an initial cluster. Import its independently verified ID as `velodb_cluster` to manage later resizing.

Both `modules/aws_byoc` and `modules/aws_byoc_existing` expose `cache_type`, `node_vcpu`, `node_count`, and `node_cache` for the initial cluster and for `additional_clusters` entries. Omit `compute_vcpu` and `cache_gb` when selecting node capacity. Existing configurations that omit node inputs retain the legacy defaults of 4 vCPUs and 100 GB. Legacy cache/CPU rules are not applied to per-node capacity.
