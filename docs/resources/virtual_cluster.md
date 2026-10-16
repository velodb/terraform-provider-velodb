---
page_title: "velodb_virtual_cluster Resource - velodb"
subcategory: ""
description: |-
  Manages an active and standby virtual cluster in a VeloDB warehouse.
---

# velodb_virtual_cluster (Resource)

Manages a virtual cluster backed by two physical compute clusters in different
availability zones. Both must already belong to the same warehouse.

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

- `velodb_warehouse.<name>.initial_cluster_id` — the warehouse's first cluster.
- `velodb_cluster.<name>.id` — a cluster managed as a separate resource.
- `module.<byoc>.initial_cluster_id` / `module.<byoc>.additional_cluster_ids[...]` — IDs returned by the BYOC modules.
- A literal ID for a cluster created outside Terraform.

Changing either desired cluster ID updates the requested pair. Terraform
refreshes `active_cluster_id` and `standby_cluster_id` from the API, so an
external failover is visible in the plan. Refresh does not automatically
switch the live pair back. To request that explicitly while leaving the
configured IDs unchanged, change `pair_reconcile_trigger` to a new integer.
Review any planned active-cluster change before applying.

Deleting the resource unmounts both physical clusters and ends virtual-cluster
routing and automatic failover. If relevant users have no default cluster,
their requests through the deleted virtual cluster will fail. Review the
destroy plan before applying.

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
  ready states in which an update or delete can proceed.
- `at_risk`: Whether the backend currently reports the pair as unable to provide
  full high availability (for example, a member is unhealthy).
- `risk_reasons`: Risk reason codes behind `at_risk`. This set can be empty while
  the virtual cluster is outside `Running`, so an empty set alone does not prove
  health — check `status` as well.
- `created_at`, `updated_at`: Creation and last-update times in RFC 3339 format
  when the API returns them.

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
