---
page_title: "Provider: VeloDB"
subcategory: ""
description: |-
  The VeloDB provider manages warehouses, compute clusters, network access, PrivateLink, and connection metadata in VeloDB Cloud.
---

# VeloDB Provider

The VeloDB provider manages VeloDB Cloud infrastructure through the VeloDB Cloud
Management API. Use it to create SaaS and advanced AWS BYOC warehouses, manage
compute clusters, control public access, register PrivateLink endpoints, and
read connection information.

To use this provider, you need a VeloDB Cloud account and a Management API key.

## Get a VeloDB Cloud API Key

Go to the VeloDB Cloud console, then open **Organization -> API Keys**.

Console URL: [https://www.velodb.cloud/organization/api-keys](https://www.velodb.cloud/organization/api-keys)

Click **Create API Key**, choose the role and expiration, then copy the generated
key. VeloDB shows the raw key only once. Keys start with `sk-`.

Store the key in an environment variable:

```shell
export VELODB_API_KEY='sk-...'
```

## API Host

The default VeloDB Cloud Management API host is:

```text
api.velodb.cloud
```

`host` is a bare hostname. Do not include `https://`; the provider adds HTTPS
for non-local hosts.

```shell
export VELODB_HOST='api.velodb.cloud'
```

## Example Usage

```terraform
terraform {
  required_providers {
    velodb = {
      source  = "velodb/velodb"
      version = "~> 1.1"
    }
  }
}

provider "velodb" {}
```

With this configuration, the provider reads `VELODB_API_KEY` and optional
`VELODB_HOST` from the environment. If you use Terraform variables instead,
set `api_key` and `host` explicitly and supply those variables separately.

For complete AWS BYOC deployments, start with the
[new-infrastructure](https://github.com/velodb/terraform-provider-velodb/tree/main/examples/aws_byoc_new_vpc) or
[existing-infrastructure](https://github.com/velodb/terraform-provider-velodb/tree/main/examples/aws_byoc_existing_infrastructure)
example.

## Resources

| Resource | Purpose |
|---|---|
| [`velodb_warehouse`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/warehouse) | Manage SaaS warehouses and advanced AWS BYOC warehouses. |
| [`velodb_byoc_credential`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/byoc_credential) | Register AWS storage and deployment credentials for BYOC. |
| [`velodb_byoc_network`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/byoc_network) | Register AWS VPC network configuration for BYOC. |
| [`velodb_encryption_key`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/encryption_key) | Register AWS KMS keys for BYOC encryption. |
| [`velodb_cluster`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/cluster) | Manage COMPUTE clusters inside a warehouse, including resize, pause, resume, and reboot. |
| [`velodb_virtual_cluster`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/virtual_cluster) | Manage an active and standby cluster pair, including role changes and health reporting. |
| [`velodb_warehouse_public_access_policy`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/warehouse_public_access_policy) | Manage public endpoint access policy and CIDR allowlists. |
| [`velodb_warehouse_private_endpoint`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/warehouse_private_endpoint) | Register and describe inbound PrivateLink endpoints for warehouse access. |
| [`velodb_private_link_endpoint_service`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/private_link_endpoint_service) | Register external endpoint services that VeloDB Cloud can access through PrivateLink. |

## Data Sources

| Data Source | Purpose |
|---|---|
| [`velodb_warehouses`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/warehouses) | List warehouses by ID, name, cloud provider, region, or deployment mode. |
| [`velodb_clusters`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/clusters) | List clusters in a warehouse by ID, name, status, type, or billing model. |
| [`velodb_warehouse_connections`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/warehouse_connections) | Read public/private endpoints, compute clusters, observer groups, and PrivateLink service names. |
| [`velodb_warehouse_versions`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/warehouse_versions) | List valid warehouse upgrade target version IDs. |
| [`velodb_private_link_endpoint_services`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/private_link_endpoint_services) | List outbound PrivateLink endpoint services and connected endpoints. |
| [`velodb_byoc_prerequisites`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/byoc_prerequisites) | Discover AWS BYOC external ID, VeloDB principal, PrivateLink service, and supported zones. |
| [`velodb_byoc_credentials`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/byoc_credentials) | List registered BYOC credentials. |
| [`velodb_byoc_network_configs`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/byoc_network_configs) | List registered BYOC network configurations. |
| [`velodb_encryption_keys`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/encryption_keys) | List registered encryption keys. |
| [`velodb_aws_assume_role_policy`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/aws_assume_role_policy) | Generate the deployment-role trust policy. |
| [`velodb_aws_crossaccount_policy`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/aws_crossaccount_policy) | Generate the deployment-role permissions policy. |
| [`velodb_aws_data_access_assume_role_policy`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/aws_data_access_assume_role_policy) | Generate the data-access role trust policy. |
| [`velodb_aws_data_access_policy`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/aws_data_access_policy) | Generate the data-access role permissions policy. |
| [`velodb_aws_kms_key_policy`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/data-sources/aws_kms_key_policy) | Generate an AWS KMS key policy for BYOC encryption. |

## Known Limitations

- New BYOC [`velodb_warehouse`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/warehouse) creation supports AWS custom infrastructure through
  `setup_mode = "advanced"`. Guided/template setup and other cloud providers are
  not supported yet.
- [`velodb_cluster`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/cluster) manages `COMPUTE` clusters. `SQL` and `OBSERVER` cluster
  types are blocked at plan time.
- A CPU resize on [`velodb_cluster`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/cluster) scales cache from its existing CPU/cache ratio, subject to a
  minimum. Set the corresponding `cache_gb` in the same apply, then apply any
  additional cache increase separately. See [cluster resizing](resources/cluster.md#resize-cpu-with-its-corresponding-cache-size).
- The current Management API does not accept `maintenance_window`,
  `upgrade_policy`, mixed-billing request fields, or legacy
  `advanced_settings` in Terraform [`velodb_warehouse`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/warehouse) or [`velodb_cluster`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/cluster) create/update requests.
- `admin_password` on [`velodb_warehouse`](https://registry.terraform.io/providers/velodb/velodb/latest/docs/resources/warehouse) is write-only in the API and is stored in Terraform state as
  a sensitive value so Terraform can detect password rotation.

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `api_key` (String, Sensitive) API key for authentication. Can also be set via the `VELODB_API_KEY` environment variable.
- `host` (String) VeloDB Cloud Management API host, without `https://`. Defaults to `api.velodb.cloud`. Can also be set via the `VELODB_HOST` environment variable.
