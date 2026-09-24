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
  zones          = ["us-east-1a", "us-east-1b", "us-east-1d"]
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

The module completes and validates all AWS prerequisites before its first
VeloDB write. AWS allocation failures stop the graph; unsupported zones,
inactive routes, and unavailable endpoints return actionable Terraform errors.
Destroy reverses the same graph: warehouse, network registration, credential,
then AWS resources.

The module creates a private subnet in each of the three zones, with the
warehouse deployed in those private subnets. Outbound internet goes through a
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
client VPCs). `bucket_force_destroy` defaults to `false` so a production bucket
with data cannot be silently emptied during destroy. Treat `name_prefix` as
immutable after creation because changing it can replace AWS and VeloDB
resources. If destroy stops with `BucketNotEmpty`, confirm the objects are no
longer needed, empty the bucket manually, and run a newly generated destroy
plan. The module does not infer whether bucket contents are shared.
