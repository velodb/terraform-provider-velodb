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

Each zone's private subnet gets a `/20` block of `vpc_cidr` derived from the AZ
letter, so leaving `subnet_cidrs` unset works for standard zones. Set the
optional `subnet_cidrs` map (zone to CIDR) to override specific zones — for
non-standard zones, to align with an external network plan, or to pin an
existing deployment's CIDRs before upgrading. See the commented block in
`terraform.tfvars.example`.

By default, `warehouse_client_cidrs` follows `vpc_cidr`, allowing clients in
the new VPC to reach warehouse TCP ports 8000-10000. Set it to `[]` to disable
direct client access, or supply a list to replace this default. Client egress
rules, network ACLs, and the warehouse access address must also permit access.

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

To also manage a virtual cluster, use a provider release containing
`velodb_virtual_cluster`, configure three availability zones, and copy the
optional resource example:

```bash
cp virtual_cluster.tf.example virtual_cluster.tf
```

The example creates a standalone physical standby cluster, then links its ID
and the BYOC initial-cluster ID to the virtual cluster. Terraform creates
everything in one apply without copying IDs.
Edit the ID expressions to select another cluster or supply an existing ID.
The module's `initial_cluster_id` is best effort at creation and null after
warehouse import or unresolved discovery. Use a verified physical ID in those
cases; older cached IDs also need verification before use.
The physical-member template includes `create_before_destroy`. If you already
copied it, add this lifecycle rule to the existing physical resource and apply
while keeping the current members and pair IDs unchanged before replacing a
member. Copying a new template does not update an existing configuration. See
[Replacing physical members](../../docs/resources/virtual_cluster.md#replacing-physical-members)
for naming, capacity, and recovery requirements.

The `virtual_cluster` output reports its configuration, live roles, and health.
It includes `risk_messages` with readable explanations and preserves the
backend codes in `risk_reasons`.
To delete only the virtual cluster, remove its `resource "velodb_virtual_cluster"
"main"` block, the `output "virtual_cluster"` block, and the `moved` block from
`virtual_cluster.tf`. Keep the physical cluster blocks in that file. Run
`terraform plan -out=delete-vc.tfplan`, check that only the virtual cluster is
marked for destruction, then run `terraform apply delete-vc.tfplan`. The
physical clusters and warehouse remain; routing and automatic failover through
the virtual cluster end. Restore the resource and output blocks from the
template to create a new virtual cluster later.

The template's `moved` block migrates existing state from the earlier counted
address `velodb_virtual_cluster.main[0]` to `velodb_virtual_cluster.main`
without recreating the cluster. Review the first plan after updating the
configuration. Remove the `moved` block along with the resource when deleting
the virtual cluster. Removing the whole `virtual_cluster.tf` file also deletes
its physical standby.

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
available by default. The data bucket is created with `force_destroy = false`,
so Terraform never deletes its objects. If destroy reports `BucketNotEmpty`,
confirm that its objects are no longer needed, empty the bucket yourself, then
create and apply a new destroy plan. Terraform will not silently purge the
objects.

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

## Node capacity and cache type

Use an updated provider build and deploy the corresponding Formation API
before applying node-based settings. Copy `terraform.tfvars.example` to
`terraform.tfvars` and uncomment the initial-cluster capacity example:

```hcl
cache_type = 0
ratio      = 4
node_vcpu  = 4
node_count = 1
node_cache = 100
```

For local NVMe, set `cache_type = 1` and omit `node_cache`. Its cache capacity
comes from the selected package. The same fields are supported in each
`additional_clusters` entry. Do not combine node inputs with legacy
`compute_vcpu`/`cache_gb`. Existing configurations keep their defaults.

The initial-cluster inputs, ratio and cache type are creation-only. Additional
cluster node-count and node-size changes require separate applies. See the
[node-capacity guide](../../docs/guides/node-capacity.md).

## Use the cluster resource directly

To add a cluster with its own resource block, copy the optional template in
this example directory:

```bash
cp cluster.tf.example cluster.tf
```

Edit `cluster.tf` to choose the cluster name, cache type, ratio, CPU, node count
and EBS cache size. The warehouse reference is already connected to this
example's module. Local NVMe requires `cache_type = 1` with `node_cache` omitted.
Use a distinct cluster name. Each physical cluster must be managed by one
resource: do not also declare the same cluster in `additional_clusters`.
The template creates a new cluster; it does not adopt the initial cluster.
Run `terraform plan` to inspect it. Applying node capacity requires the matching
Formation API deployment. Removing `cluster.tf` later plans deletion of this
extra cluster while keeping the module's warehouse configuration.

CPU sizes must match a package available for the region, cache type and ratio.
Change node count separately from node CPU/cache size.
