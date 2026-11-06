# Node-based cluster capacity

Node capacity requires a provider build and Formation API deployment that both support `nodeVcpu`, `nodeCount`, and `nodeCache`. Storage type has a separate compatibility requirement. Provider builds that send `cacheStorageType` require an API deployment that accepts and returns that name. Earlier node-capacity APIs used `cacheType`. Verify the API deployment before creating or updating clusters with explicit storage settings.

A node is one compute machine. `node_vcpu` and `node_cache` describe each machine; `node_count` is the whole-cluster machine count. For example, three nodes with 8 vCPUs and 200 GB cache each have 24 total vCPUs and 600 GB total cache.

## Upgrade from `cache_type`

Update resource and data-source references and BYOC module inputs from
`cache_type` to `cache_storage_type`. The values remain `0` for EBS and `1`
for local NVMe. The provider does not automatically migrate the old state
attribute or the module snapshot key. If existing state uses the old name,
back it up and review the plan before applying. Resolve any proposed cluster
replacement or storage change before proceeding.

## Create

Use exactly one capacity input family:

- Legacy: `compute_vcpu` and `cache_gb`.
- EBS nodes: `node_vcpu`, `node_count`, and `node_cache`; optionally set `cache_storage_type = 0`.
- Local NVMe nodes: `cache_storage_type = 1`, `node_vcpu`, and `node_count`; omit `node_cache`.

EBS is the creation default when cache_storage_type is omitted. Capacity values must be positive integers; package sizing, cache steps and stock remain server-validated. For local NVMe, `node_cache` is computed from the selected package and cannot be configured, even if it matches the current size. The API ignores supplied local cache sizes; the provider rejects explicit values during planning to prevent an ignored configuration from disagreeing with Terraform state. Cache storage type is creation-only; changing it is rejected without proposing replacement.

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
  cache_storage_type   = 1
  ratio        = 4
  node_vcpu    = 4
  node_count   = 2
  # node_cache is read from the API, not configured.
}
```

Cluster creation saves the returned ID before waiting for `Running` or pausing. Failed waits stop the apply and retain the last confirmed state. Terraform may mark a failed creation for replacement; inspect the saved cluster and review the next plan before retrying.

## Update

For EBS, change `node_vcpu` and, when required by the selected package, `node_cache` together. Change `node_count` in a separate apply. The provider rejects a single apply that changes both count and node size/cache. Cache-only shrinking is unsupported.

For Local storage (`cache_storage_type = 1`), omit `node_cache`, even if it matches the current size. Cache capacity is determined automatically by the node package. For vertical scaling, change only `node_vcpu` and leave `node_count` unchanged; change `node_count` in a separate apply. The provider reads the resulting `node_cache` from the API; no cache size or cache storage type is sent in Local-storage update requests. Resizing preserves the existing memory-to-vCPU ratio.

The provider waits for both a stable status and the requested geometry, so an old `Running` response cannot prematurely complete a resize. On timeout, refresh before retrying: an accepted operation can still finish. Backend cooldowns can require waiting before the next separate apply.

When combining a state change with node resizing, the provider resumes and waits for `Running` before resizing, or completes the resize before pausing. A reboot completion error stops subsequent updates. If a later step fails, Terraform retains confirmed capacity changes and accepted reboot triggers; refresh before retrying.

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

`velodb_warehouse.initial_cluster` accepts the same two creation families. For Terraform-created warehouses with a saved initial_cluster_id, CPU/cache and node count can be resized through this block using the same separate-apply rules above. Applied capacity is read from that exact cluster, including package-defined local node_cache. Zone, ratio, cache storage type and auto-pause stay creation-only. Errors retain confirmed capacity and the warehouse/cluster IDs; refresh before retrying. If refresh confirms the initial cluster was deleted (HTTP 404), its cached ID is cleared with a warning while the warehouse and last known capacity are retained. Warehouse changes and deletion remain available; capacity edits fail until a verified cluster is managed separately. Permission and temporary API failures still return errors. Warehouse import does not reconstruct or adopt an initial cluster. Remove the block before transferring management to a separate velodb_cluster resource, then import its independently verified ID.

Both `modules/aws_byoc` and `modules/aws_byoc_existing` expose `cache_storage_type`, `node_vcpu`, `node_count`, and `node_cache` for the initial cluster and for `additional_clusters` entries. Omit `compute_vcpu` and `cache_gb` when selecting node capacity. Existing configurations that omit node inputs retain the legacy defaults of 4 vCPUs and 100 GB. Legacy cache/CPU rules are not applied to per-node capacity.

Existing warehouse states require a one-time capacity adoption before initial-cluster resizing. Match CPU/cache/node inputs to API values and apply once; this records adoption without a resize. The provider rejects mismatches at plan and rechecks at apply. No configuration flag is required. Do not adopt capacity already owned by a separate `velodb_cluster`; remove the warehouse block to relinquish ownership. After adoption, console changes are ordinary drift. A post-create capacity-read failure warns and preserves IDs; later refresh must succeed before adoption. Missing IDs are never inferred during migration.
