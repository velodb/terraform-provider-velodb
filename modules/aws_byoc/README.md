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

The module uses one NAT gateway to keep the basic deployment affordable. Use a
separate high-availability network module when one NAT gateway per zone is
required. `bucket_force_destroy` defaults to `false` so a production bucket
with data cannot be silently emptied during destroy. Treat `name_prefix` as
immutable after creation because changing it can replace AWS and VeloDB
resources. If destroy stops with `BucketNotEmpty`, confirm the objects are no
longer needed, empty the bucket manually, and run a newly generated destroy
plan. The module does not infer whether bucket contents are shared.
