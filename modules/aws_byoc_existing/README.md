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
cannot be changed after the warehouse exists. Set `engine_version` (e.g.
`"26.1"`, `major.minor`) to pin the initial engine version; leave it unset to
let the management API pick the default. It is also create-only — use
`core_version_id` to upgrade an existing warehouse.

Destroy removes the warehouse and its VeloDB registrations. All AWS resources
remain and must be removed separately by their owner if no longer needed.
