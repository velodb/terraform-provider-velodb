---
page_title: "velodb_virtual_cluster Resource - velodb"
subcategory: ""
description: |-
  Manages an active and standby virtual cluster in a VeloDB warehouse.
---

# velodb_virtual_cluster (Resource)

Manages a virtual cluster backed by two physical compute clusters in different
availability zones. Both must already belong to the same warehouse.

At creation, both members must be `Running` and neither may already belong to
another virtual cluster. Warehouse eligibility and supported engine-version
requirements also apply. A rejected create can indicate an occupied or stopped
member; older Cloud Manager deployments may report only a generic warehouse
eligibility error for these cases.

The two members are ordinary compute-cluster IDs — any two distinct clusters in
the warehouse. One takes the active role and the other standby; the role is
independent of which cluster is the warehouse's initial cluster.

```terraform
resource "velodb_virtual_cluster" "main" {
  warehouse_id = velodb_warehouse.main.id
  name         = "production_vc"

  # Any two distinct compute clusters in this warehouse. Here the warehouse's
  # first cluster is active and a separately managed cluster is standby —
  # either ID can be any compute cluster in the warehouse.
  desired_active_cluster_id  = velodb_warehouse.main.initial_cluster_id
  desired_standby_cluster_id = velodb_cluster.standby.id
}
```

Supply each member ID from whatever fits your configuration:

- `velodb_warehouse.<name>.initial_cluster_id` — a best-effort creation-time ID, when available.
- `velodb_cluster.<name>.id` — a cluster managed as a separate resource.
- `module.<byoc>.initial_cluster_id` / `module.<byoc>.additional_cluster_ids[...]` — IDs returned by the BYOC modules.
- A literal ID for a cluster created outside Terraform.

The `initial_cluster_id` output is null after warehouse import or unresolved
creation-time discovery. After warehouse import, treat all existing compute
clusters as ordinary physical clusters. No member needs an initial role.
Reference a managed `velodb_cluster` resource's `.id`, explicitly select an
existing cluster with the [`velodb_clusters` data source](../data-sources/clusters.md),
or supply an independently verified physical ID. Import a physical cluster only
if you want Terraform to manage it; import is not required just to reference a
member. Verify older cached values before using them. See
[Managing the Initial Cluster](warehouse.md#managing-the-initial-cluster).

Changing either desired cluster ID updates the requested pair. Terraform
refreshes `active_cluster_id` and `standby_cluster_id` from the API, so an
external failover is visible in the plan. Refresh does not automatically
switch the live pair back. To request that explicitly while leaving the
configured IDs unchanged, change `pair_reconcile_trigger` to a new integer.
Review any planned active-cluster change before applying.

Avoid managing the same virtual cluster from independent Terraform states.
If a rename or pair update observes a stable value different from both the
value read before the request and the requested value, the provider reports a
concurrent-modification error instead of waiting for the update timeout. It
saves the observed remote state and retains the previous desired pair and
reconciliation trigger for an unverified pair change. Refresh and review a new
plan before retrying; the provider does not automatically repeat the write.
Reads that still show the pre-update value continue polling because they may
reflect normal propagation delay. This detection does not prevent stale
writes or detect an intervening update that restores the original value.

A member promoted to active must be `Running`, including when swapping the
existing pair. A swap can demote a stopped active member while promoting a
running standby. A replacement standby must also be `Running`.

Deleting the resource unmounts both physical clusters and ends virtual-cluster
routing and automatic failover. If relevant users have no default cluster,
their requests through the deleted virtual cluster will fail. Review the
destroy plan before applying.

If deletion times out or fails, Terraform retains the virtual-cluster ID until
GET confirms that the target no longer exists. Running destroy again submits
DELETE even when the virtual cluster is already `Deleting`: the backend
acknowledges an active deletion task or queues another cleanup attempt after a
failed task. Terraform then waits for confirmed absence; an accepted DELETE or
`Deleted` status alone does not remove the resource from state.

Retry recovery requires the Cloud Manager failed-deletion retry support and the
Formation worker that handles already-removed remote configuration. Deploy the
Formation worker first, then Cloud Manager, before relying on this behavior.
A failed deletion does not restore routing or automatic failover; inspect the
backend task if repeated attempts fail.

## Replacing physical members

A bound physical cluster cannot be deleted. Without lifecycle ordering,
Terraform can try to delete the old member before updating the virtual-cluster
pair and receive `409 OperationConflict`; retrying the same plan can fail again.
The provider cannot reorder Terraform's dependency graph from its delete call.

Set this on each independently managed physical member (`velodb_cluster`):

```terraform
lifecycle {
  create_before_destroy = true
}
```

Both BYOC modules include this rule for `additional_clusters`. For an existing
deployment, first apply the updated module or lifecycle rule while keeping the
current members and desired pair IDs unchanged. Terraform records the lifecycle
rule in state even if no API writes are required. Do this **before** removing an
outgoing declaration or map key; adding the rule only to its replacement does
not protect the outgoing member.

For a replacement at the same resource address, keep the rule and change the
replacement-triggering attribute (for example, `zone`) together with the name.
Choose a name distinct from the old name ignoring case, allow capacity for both
physical clusters, and keep the replacement in a zone different from its partner.
Changing only `name` performs a rename, not a replacement. Reference the managed
member's `.id` in the virtual-cluster pair so Terraform can order creation,
pair update, then deletion of the old member. Literal IDs do not establish this
dependency. Review the plan: the virtual cluster must retain its ID.

For migration to a new resource address or `additional_clusters` key, the
outgoing member must already have this lifecycle rule recorded in state. If
that cannot be established, use two applies: keep both physical declarations,
create the new member and update the pair, then remove the old declaration
after the pair update completes. This also works when the new member already
exists. The warehouse's initial cluster has its own lifecycle; this rule does
not make replacing the warehouse safe.

If an apply already failed to delete a bound member:

1. Keep or restore the old physical declaration or map entry with its original
   settings. Retain any replacement already created and tracked in state.
2. Point the desired pair at the replacement and apply while both physical
   declarations remain. Verify the live pair no longer contains the old ID.
3. Remove the detached old declaration and apply again. Confirm the virtual
   cluster retains its ID and the final plan is empty.

Do not remove state, recreate the virtual cluster, or manually delete a bound
member to work around ordering. Resolve other reported conflicts separately.

## Arguments

- `warehouse_id` (required): Parent warehouse ID. Moving to another warehouse
  is not supported in place.
- `name` (required): Virtual-cluster name, up to 32 characters, starting with a
  letter and containing letters, numbers, and nonconsecutive underscores.
- `desired_active_cluster_id` (required): Requested active physical-cluster ID.
- `desired_standby_cluster_id` (required): Requested standby physical-cluster ID.
- `pair_reconcile_trigger` (optional): Change to request reapplication of the
  desired pair after an external role switch.
- `timeouts` (optional): `create`, `update`, and `delete` operation timeouts.

## Read-only Attributes

These are read from the API on every refresh and cannot be set in configuration:

- `id`: Virtual-cluster identifier. Used with `warehouse_id` for import.
- `active_cluster_id`: Physical cluster currently serving traffic. Reflects the
  live role, which can differ from `desired_active_cluster_id` after a failover.
- `standby_cluster_id`: Physical cluster currently held in reserve.
- `status`: Lifecycle status reported by the backend, such as `Creating`,
  `Running`, `AtRisk`, `Updating`, or `Deleting`. `Running` and `AtRisk` are the
  ready states in which an update or initial delete can proceed. A destroy retry
  can also resume cleanup from `Deleting`.
- `at_risk`: Whether the backend currently reports the pair as unable to provide
  full high availability (for example, a member is unhealthy).
- `risk_reasons`: Risk reason codes behind `at_risk`. This set can be empty while
  the virtual cluster is outside `Running`, so an empty set alone does not prove
  health — check `status` as well.
- `risk_messages`: Human-readable explanations derived from `risk_reasons` by
  the provider. These do not include capacity numbers or change the backend's
  health assessment. Use `risk_reasons` for code-based checks.
- `created_at`, `updated_at`: Creation and last-update times in RFC 3339 format
  when the API returns them.

## Risk reasons

The backend supplies `risk_reasons`; the provider exposes them as a set of
strings and supplies readable phrases in `risk_messages`. For example:

```hcl
at_risk       = true
risk_reasons  = ["CacheResourceLow"]
risk_messages = ["Standby cache capacity is lower than active cache capacity."]
```

To show the phrases in your own configuration:

```hcl
output "virtual_cluster_risk_messages" {
  value = velodb_virtual_cluster.main.risk_messages
}
```

Messages refresh with the observed risks during create, update, import, and
read. A stopped standby is described as `Standby cluster is not running
(status: Stopped).` Unknown backend reason codes remain unchanged in
`risk_reasons` and appear in a message such as `Cloud Manager reported risk
"FutureRisk".` Empty messages, like empty reasons, do not prove health outside
`Running`; inspect `status` and `at_risk` too. Existing outputs that select only
`risk_reasons` must also select `risk_messages` to display the phrases.

Current Cloud Manager deployments report specific causes:

| Reason | Meaning |
|---|---|
| `ActiveCluster<Status>`, `StandbyCluster<Status>` | That member is not running, for example `StandbyClusterStopped`, `StandbyClusterStarting`, or `ActiveClusterSuspended`. |
| `ActiveClusterMissing`, `StandbyClusterMissing` | The member could not be found. |
| `ActiveClusterStatusUnknown`, `StandbyClusterStatusUnknown` | The member's status is missing or unrecognized. |
| `ActiveResourcesMissing`, `StandbyResourcesMissing` | The member has no capacity record. |
| `ActiveComputeCapacityUnknown`, `StandbyComputeCapacityUnknown` | CPU or node-count information needed to evaluate compute capacity is missing. |
| `ActiveCacheCapacityUnknown`, `StandbyCacheCapacityUnknown` | Disk or node-count information needed to evaluate cache capacity is missing. |
| `ArchInconsistent` | The members use different CPU architectures. |
| `ComputeResourceLow` | Total standby CPU capacity is below active CPU capacity. |
| `CacheResourceLow` | Total standby cache capacity is below active cache capacity. |
| `BackendRiskReported` | The backend reports an additional risk without a more specific public cause. |

For example, a stopped standby can produce `at_risk = true` and
`risk_reasons = ["StandbyClusterStopped"]` while the VC lifecycle remains
`Running`. Multiple issues can be reported together. These codes depend on the
deployed backend, not the provider version; older deployments can still return
the broad `ServiceUnavailable` reason. Accept additional reason strings as the
backend evolves, and check lifecycle status before interpreting an empty set.

## Import

Import with `warehouse_id/virtual_cluster_id`:

```shell
terraform import velodb_virtual_cluster.main WAREHOUSE_ID/VIRTUAL_CLUSTER_ID
```

If creation returns `202 Processing`, the provider retries with the same
`RequestId` and request body, subject to its existing retry limit and timeouts.
If it still cannot obtain the virtual-cluster ID, the error includes the
request ID for investigation. Check the warehouse for the requested name and
members, and import only a confirmed match before attempting creation again.
If no match is visible, investigate the request ID with support; a failed
create can be hidden from GET and list responses, so absence does not prove
cleanup completed. These retries do not provide complete recovery when the
backend loses the association between the request and the created cluster.

## Linking to BYOC module outputs

For managed AWS BYOC deployments, the
[`aws_byoc`](https://github.com/velodb/terraform-provider-velodb/tree/main/modules/aws_byoc)
and [`aws_byoc_existing`](https://github.com/velodb/terraform-provider-velodb/tree/main/modules/aws_byoc_existing)
modules expose the warehouse and initial-cluster IDs. Additional physical
clusters can be declared as standalone resources:

```terraform
resource "velodb_cluster" "standby" {
  warehouse_id = module.byoc.warehouse_id
  name         = "production_standby"
  cluster_type = "COMPUTE"
  zone         = "us-east-1b"
  compute_vcpu = 4
  cache_gb     = 100

  lifecycle {
    create_before_destroy = true
  }
}

resource "velodb_virtual_cluster" "main" {
  warehouse_id               = module.byoc.warehouse_id
  name                       = "production_vc"
  desired_active_cluster_id  = module.byoc.initial_cluster_id
  desired_standby_cluster_id = velodb_cluster.standby.id
}
```

Terraform creates the warehouse and physical clusters before the virtual
cluster in the same apply. The IDs can also come from separate resources,
variables, or existing clusters. Both clusters must belong to the selected
warehouse and be in different availability zones.

## Moving from the bundled module resource

If the earlier module implementation already manages your virtual cluster,
add the separate resource and move its state before applying. For example:

```terraform
moved {
  from = module.byoc.velodb_virtual_cluster.this[0]
  to   = velodb_virtual_cluster.main
}
```

Substitute your actual resource addresses and preserve the configured name,
desired IDs, and reconciliation trigger. Verify the plan shows a state move
without virtual-cluster creation or destruction. Never remove the old state
entry to recreate an existing virtual cluster.
