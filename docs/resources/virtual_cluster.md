---
page_title: "velodb_virtual_cluster Resource - velodb"
subcategory: ""
description: |-
  Manages an active and standby virtual cluster in a VeloDB warehouse.
---

# velodb_virtual_cluster (Resource)

Manages a virtual cluster backed by two physical compute clusters in different
availability zones. The clusters must already belong to the same warehouse.

```terraform
resource "velodb_virtual_cluster" "main" {
  warehouse_id               = velodb_warehouse.main.id
  name                       = "production_vc"
  desired_active_cluster_id  = velodb_warehouse.main.initial_cluster_id
  desired_standby_cluster_id = velodb_cluster.standby.id
}
```

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

`id`, `active_cluster_id`, `standby_cluster_id`, `status`, `at_risk`,
`risk_reasons`, `created_at`, and `updated_at` are read from the API.
`risk_reasons` can be empty while the virtual cluster is outside `Running`;
an empty set alone does not prove health.

## Import

Import with `warehouse_id/virtual_cluster_id`:

```shell
terraform import velodb_virtual_cluster.main WAREHOUSE_ID/VIRTUAL_CLUSTER_ID
```

For managed AWS BYOC deployments, the
[`aws_byoc`](https://github.com/velodb/terraform-provider-velodb/tree/main/modules/aws_byoc)
and [`aws_byoc_existing`](https://github.com/velodb/terraform-provider-velodb/tree/main/modules/aws_byoc_existing)
modules can create the member clusters and virtual cluster together.
