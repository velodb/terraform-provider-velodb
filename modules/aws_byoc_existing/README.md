# AWS BYOC existing-infrastructure module

Creates VeloDB credential and network registrations plus a warehouse using
existing AWS infrastructure. AWS resources are read for validation only; this
module does not create, import, modify, or delete them.

The module verifies the bucket, VPC, subnets, security group, endpoint, IAM
instance profile, and IAM role before its first VeloDB create request.

Do not run `terraform import` for shared AWS resources in this warehouse state.
Import transfers lifecycle management to Terraform and makes a resource subject
to modification, replacement, and deletion.

Required existing infrastructure:

- An S3 bucket.
- A VPC with DNS support and DNS hostnames enabled.
- One private subnet for single-zone deployment or three private subnets for
  cross-zone deployment. Each must be in a VeloDB-supported availability zone
  with a default route through an available NAT gateway.
- A warehouse security group.
- A VeloDB interface VPC endpoint in the same VPC.
- A data-access IAM instance profile and deployment IAM role with the required
  VeloDB policies.

Create any number of additional clusters with a stable map key:

```hcl
additional_clusters = {
  analytics = {
    name         = "production_analytics"
    compute_vcpu = 8
    cache_gb     = 200
  }
}
```

`zone` is optional and defaults to the first configured zone. Change `name` to
rename a cluster without changing its Terraform identity. Removing an entry
deletes only that additional cluster.

Set `tags` to apply extra key/value pairs to the VeloDB warehouse (it gets a
`managed-by = terraform` tag by default). Warehouse tags are create-only and
cannot be changed after the warehouse exists. Set `initial_core_version` (e.g.
`"26.1"`, `major.minor`) to pin the initial core version; leave it unset to
let the management API pick the default. It is also create-only — use
`core_version_id` to upgrade an existing warehouse.

Set `public_access_policy` to apply an initial public access policy at creation: an
object with `policy` (`DENY_ALL`, `ALLOW_ALL`, or `ALLOWLIST_ONLY`) and, for
`ALLOWLIST_ONLY`, a list of `rules` (`cidr` plus optional `description`). It is
create-only; manage the policy afterward with the
`velodb_warehouse_public_access_policy` resource. Leave it unset to let the
management API pick the default. For example:

```hcl
public_access_policy = {
  policy = "ALLOWLIST_ONLY"
  rules = [
    { cidr = "203.0.113.0/24", description = "office" },
  ]
}
```

Destroy removes the warehouse and its VeloDB registrations. All AWS resources
remain and must be removed separately by their owner if no longer needed.

## Immutable warehouse infrastructure

Once created, warehouse infrastructure inputs cannot be edited in place. The
module rejects changes to `bucket_name`, `region`, network placement, and
`create_tde_encryption_key`, `create_ebs_encryption_key`, `tde_kms_key_arn`, and
`ebs_kms_key_arn`. The new-VPC module also freezes `vpc_cidr`, `zones`, and
`subnet_cidrs`; the existing-infrastructure module freezes VPC, subnet, security
group, endpoint, and IAM credential references. Restore the original values
when a plan reports an immutable-input error. Provision a separate warehouse
for a migration instead of replacing the existing warehouse through input edits.

When upgrading a deployment created before these guards existed, first apply
with all infrastructure inputs unchanged to record their baseline. Do not
combine that upgrade with infrastructure edits. The provider also rejects
changes to existing warehouse bindings, including unknown IDs produced by
upstream replacement plans. An unknown binding must be resolved without
replacing infrastructure already used by the warehouse before planning again.

These checks do not prevent an explicit `terraform destroy` or removal of the
module from configuration. They are immutability checks, not deletion protection.
