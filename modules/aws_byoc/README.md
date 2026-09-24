# AWS BYOC module

Creates a new single-zone or three-zone AWS VPC and the AWS IAM, S3,
networking, security, PrivateLink, VeloDB registration, and warehouse resources
required for BYOC. Three zones remain the recommended production default.
All resources created by this module are managed in the same Terraform state
and are subject to modification, replacement, and deletion.

See the complete [`aws_byoc_new_vpc` example](../../examples/aws_byoc_new_vpc)
for a ready-to-copy user configuration.

```hcl
provider "aws" {
  region = "us-east-1"
}

provider "velodb" {}

module "velodb_byoc" {
  source = "./modules/aws_byoc"

  region         = "us-east-1"
  name_prefix    = "production"
  bucket_name    = "globally-unique-velodb-bucket"
  zones          = ["us-east-1a", "us-east-1b", "us-east-1c"]
  admin_password = var.admin_password

  additional_clusters = {
    analytics = {
      name         = "production_analytics"
      compute_vcpu = 8
      cache_gb     = 200
    }
  }
}
```

Add any number of entries to `additional_clusters`. Keep each map key stable;
change `name` when renaming a cluster so Terraform updates it instead of
replacing it. `zone` is optional and defaults to the first configured zone.
Removing an entry deletes only that additional cluster.

Set `tags` to apply extra key/value pairs to the AWS resources and the VeloDB
warehouse (all get a `managed-by = terraform` tag by default). Warehouse tags
are create-only, so they cannot be changed after the warehouse exists. Set
`engine_version` (e.g. `"26.1"`, `major.minor`) to pin the initial engine
version; leave it unset to let the management API pick the default. It is also
create-only — use `core_version_id` to upgrade an existing warehouse.

The module completes and validates all AWS prerequisites before its first
VeloDB write. AWS allocation failures stop the graph; unsupported zones,
inactive routes, and unavailable endpoints return actionable Terraform errors.
Destroy reverses the same graph: warehouse, network registration, credential,
then AWS resources. Every underlying AWS resource the running warehouse relies
on -- the S3 gateway endpoint, the PrivateLink endpoint, the NAT default route,
and the security-group rules -- is anchored to the credential, so it is torn
down only after the warehouse has finished deleting rather than in parallel with
it.

The module creates a private subnet in each configured zone, with the
warehouse deployed in those private subnets. Each subnet gets a `/20` block of
`vpc_cidr` keyed on the availability-zone letter (`us-east-1a` → block 1,
`us-east-1b` → block 2, and so on), so a zone always maps to the same CIDR
regardless of its position in `zones`. This keeps zone changes safe: swapping
one zone for another (for example `us-east-1b` → `us-east-1c`) gives the new
subnet its own free block instead of colliding with the one being removed.

Set `subnet_cidrs` to override the derived block for specific zones — a map of
zone to CIDR, merged over the defaults, where unset zones keep their derived
block. Use it to place subnets for non-standard zones (Local Zones, Wavelength,
or a region with more than eight availability zones, none of which the derived
`<region><letter>` scheme with letters `a`–`h` supports), or to align subnets
with an external network plan. An explicit override also bypasses the letter
lookup entirely for that zone.

Because the derived mapping changed in this release, upgrading an existing
deployment whose `zones` list skips a letter (for example
`["us-east-1a", "us-east-1b", "us-east-1d"]`) reassigns the affected subnet CIDR
and triggers a one-time subnet, network, and warehouse replacement on the next
apply even if `zones` is unchanged. To upgrade in place without that
replacement, pin the current CIDRs first, for example:

```hcl
subnet_cidrs = {
  "us-east-1d" = "10.57.48.0/20" # the block this zone had under the old scheme
}
```

Outbound internet goes through a
single [regional NAT gateway](https://docs.aws.amazon.com/vpc/latest/userguide/nat-gateways-regional.html)
(`availability_mode = "regional"`, automatic mode), which is multi-AZ and
highly available by default: it expands and contracts across AZs with the
workload and keeps zonal affinity, so an AZ outage does not cut egress for the
surviving AZs. It needs no public subnet and manages its own Elastic IPs. This
requires the `hashicorp/aws` provider `>= 6.24.0`; regional NAT is unavailable
in AWS GovCloud (US) and China Regions, where a zonal NAT per AZ is needed
instead.

By default the warehouse security group only allows internal traffic and
PrivateLink access. Set `warehouse_client_cidrs` to the CIDR blocks of any VPCs
that must reach the warehouse directly on ports 8000-10000 (for example, peered
client VPCs). The data bucket is created with `force_destroy = false` so
Terraform never deletes its objects: the module creates the bucket, but the
warehouse writes the data, so destroy will not silently empty it. Treat
`name_prefix` as
immutable after creation because changing it can replace AWS and VeloDB
resources. If destroy stops with `BucketNotEmpty`, confirm the objects are no
longer needed, empty the bucket manually, and run a newly generated destroy
plan. The module does not infer whether bucket contents are shared.
