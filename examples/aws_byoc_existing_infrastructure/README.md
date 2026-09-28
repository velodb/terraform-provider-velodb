# Create an AWS BYOC warehouse using existing infrastructure

This example registers existing AWS infrastructure with VeloDB and creates a
BYOC warehouse. It uses `modules/aws_byoc_existing` and does not require the
separate `byoc-terraform` repository.

The module reads AWS resources to validate them, but does not create, import,
modify, or delete them. Terraform manages only the VeloDB credential and
network registrations and the warehouse.

## Prerequisites

- Terraform 1.5 or newer.
- VeloDB provider 1.1.8 or newer.
- AWS credentials that can read the referenced resources.
- A VeloDB API key that can manage BYOC warehouses.
- An existing S3 bucket, VPC, one or three private subnets with working NAT
  gateway egress, warehouse security group, VeloDB interface VPC endpoint,
  data-access instance profile, and deployment IAM role.

## Configure authentication

```bash
export VELODB_HOST='api.velodb.cloud'
export VELODB_API_KEY='replace-with-your-api-key'
export TF_VAR_admin_password='replace-with-a-strong-password'
```

Configure AWS authentication using an AWS profile or another standard AWS
provider authentication method.

## Configure the deployment

```bash
cp terraform.tfvars.example terraform.tfvars
```

Replace every placeholder in `terraform.tfvars` with an existing resource ID,
ARN, or name. The three subnet map keys must be their actual availability zones.
Treat `name_prefix` as immutable after the first apply.

To create more compute clusters, add entries to `additional_clusters` in
`terraform.tfvars`:

```hcl
additional_clusters = {
  analytics = {
    name         = "production_analytics"
    compute_vcpu = 8
    cache_gb     = 200
  }
}
```

Keep the map keys stable. `zone` defaults to the first configured zone. Change
`name` to rename a cluster; remove a map entry to delete only that cluster.

Do not run `terraform import` for the existing AWS resources. Importing them
would make Terraform manage their lifecycle, including replacement or deletion.

## Create the warehouse

```bash
terraform init
terraform plan -out=tfplan
terraform apply tfplan
terraform plan
```

The first plan validates the existing AWS resources before it sends any create
request to VeloDB. The final plan must report `No changes`. Do not treat the
deployment as successful until the warehouse reaches `Running`.

## Destroy the warehouse

```bash
terraform plan -destroy -out=destroy.tfplan
terraform apply destroy.tfplan
terraform state list
```

Destroy removes the warehouse and the VeloDB network and credential
registrations. It does not delete the referenced VPC, subnets, endpoint,
security group, IAM resources, or S3 bucket. Their owner must remove them
separately if they are no longer needed.

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
