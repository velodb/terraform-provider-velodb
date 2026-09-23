# Create an AWS BYOC warehouse in a new VPC

This example creates a complete VeloDB AWS BYOC deployment in a new VPC. It
uses the reusable `modules/aws_byoc` module and does not require the separate
`byoc-terraform` repository.

The deployment includes AWS IAM, S3, a VPC, one public subnet, three private
subnets, a NAT gateway, routing, security groups, VPC endpoints, VeloDB
credential and network registrations, and a warehouse with an initial cluster.

## Prerequisites

- Terraform 1.5 or newer.
- VeloDB provider 1.1.8 or newer for normal user installations.
- AWS credentials with permission to create the resources above.
- A VeloDB API key with permission to manage BYOC warehouses.
- Three availability zones returned by the VeloDB BYOC discovery API.
- Capacity for one Elastic IP and one NAT gateway.

## Configure authentication

Set the VeloDB API host and API key without putting secrets in Terraform files:

```bash
export VELODB_HOST='api.velodb.cloud'
export VELODB_API_KEY='replace-with-your-api-key'
export TF_VAR_admin_password='replace-with-a-strong-password'
```

Configure AWS authentication using an AWS profile or another standard AWS
provider authentication method.

## Configure the deployment

Copy the example variables:

```bash
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` and provide a globally unique bucket name, a resource
name prefix, a non-overlapping VPC CIDR, and three zones supported by VeloDB in
the selected region. Treat `name_prefix` as immutable after the first apply;
changing it can replace AWS and VeloDB resources.

## Create the warehouse

```bash
terraform init
terraform plan -out=tfplan
terraform apply tfplan
terraform plan
```

The final plan must report `No changes`. Warehouse creation can take several
minutes. Do not treat the deployment as successful until the warehouse reaches
`Running`.

## Destroy the deployment

```bash
terraform plan -destroy -out=destroy.tfplan
terraform apply destroy.tfplan
terraform state list
```

The final command must return no resources. The module waits for warehouse
deletion before removing VeloDB registrations and AWS dependencies.

The module intentionally uses one NAT gateway. The default
`bucket_force_destroy = false` protects a nonempty production bucket from
automatic deletion.
