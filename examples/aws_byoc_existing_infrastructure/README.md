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
- An existing S3 bucket, VPC, three private subnets with working NAT gateway
  egress, warehouse security group, VeloDB interface VPC endpoint, data-access
  instance profile, and deployment IAM role.

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
