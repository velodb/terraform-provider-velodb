# Create an AWS BYOC warehouse in a new VPC

This example creates a complete VeloDB AWS BYOC deployment in a new VPC. It
uses the reusable `modules/aws_byoc` module and does not require the separate
`byoc-terraform` repository.

Use this workflow only when Terraform should own the new AWS infrastructure.
Resources created by this configuration are recorded in its state and are
subject to modification, replacement, and deletion by Terraform. To use
shared or separately managed AWS resources instead, use the
[`aws_byoc_existing_infrastructure`](../aws_byoc_existing_infrastructure)
example.

The deployment includes AWS IAM, S3, a VPC, one or three private subnets, a regional
NAT gateway, routing, security groups, VPC endpoints, VeloDB credential and
network registrations, and a warehouse with an initial cluster.

## Prerequisites

- Terraform 1.5 or newer.
- AWS provider 6.24.0 or newer (required for the regional NAT gateway).
- VeloDB provider 1.1.8 or newer for normal user installations.
- AWS credentials with permission to create the resources above.
- A VeloDB API key with permission to manage BYOC warehouses.
- One or three availability zones returned by the VeloDB BYOC discovery API.
- An AWS Region that supports regional NAT gateways (all commercial Regions;
  not AWS GovCloud (US) or China Regions).

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
name prefix, a non-overlapping VPC CIDR, and either one or three zones supported
by VeloDB in the selected region. Three zones are recommended for production.
Treat `name_prefix` as immutable after the first apply; changing it can replace
AWS and VeloDB resources.

To create more compute clusters, add entries to `additional_clusters` in
`terraform.tfvars`:

```hcl
additional_clusters = {
  analytics = {
    name         = "production_analytics"
    compute_vcpu = 8
    cache_gb     = 200
  }
  etl = {
    name         = "production_etl"
    zone         = "us-east-1a"
    compute_vcpu = 16
    cache_gb     = 400
  }
}
```

Keep the map keys stable. Change `name` to rename a cluster; remove a map entry
to delete only that additional cluster.

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

The module uses a single regional NAT gateway, which is multi-AZ and highly
available by default. The default
`bucket_force_destroy = false` protects a nonempty production bucket from
automatic deletion. If destroy reports `BucketNotEmpty`, confirm that its
objects are no longer needed, empty the bucket yourself, then create and apply
a new destroy plan. Terraform will not silently purge the objects.
