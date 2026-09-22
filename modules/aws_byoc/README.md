# AWS BYOC module

Creates a new three-zone AWS VPC and the AWS IAM, S3, networking, security,
PrivateLink, VeloDB registration, and warehouse resources required for BYOC.

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
}
```

The module completes and validates all AWS prerequisites before its first
VeloDB write. AWS allocation failures stop the graph; unsupported zones,
inactive routes, and unavailable endpoints return actionable Terraform errors.
Destroy reverses the same graph: warehouse, network registration, credential,
then AWS resources.

The module uses one NAT gateway to keep the basic deployment affordable. Use a
separate high-availability network module when one NAT gateway per zone is
required. `bucket_force_destroy` defaults to `false` so a production bucket
with data cannot be silently emptied during destroy.
