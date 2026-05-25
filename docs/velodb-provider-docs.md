# VeloDB Terraform Provider

The VeloDB Terraform provider manages VeloDB Cloud warehouses, compute clusters,
public access policy, PrivateLink registrations, and connection metadata through
the VeloDB Cloud Management API.

## Get a VeloDB Cloud API key

Go to the VeloDB Cloud console, then open **Organization -> API Keys**.

Console URL: <https://www.velodb.cloud/organization/api-keys>

Click **Create API Key**, choose the role and expiration, then copy the generated
key. VeloDB shows the raw key only once. Keys start with `sk-`.

```bash
export VELODB_API_KEY='sk-...'
```

## API host

The default VeloDB Cloud Management API host is:

```text
api.velodb.cloud
```

`host` is a bare hostname. Do not include `https://`.

```bash
export VELODB_HOST='api.velodb.cloud'
```

## Provider configuration

```terraform
terraform {
  required_providers {
    velodb = {
      source  = "velodb/velodb"
      version = "~> 1.1"
    }
  }
}

provider "velodb" {
  host    = var.velodb_host
  api_key = var.velodb_api_key
}
```

## Resources

- `velodb_warehouse`
- `velodb_cluster`
- `velodb_warehouse_public_access_policy`
- `velodb_warehouse_private_endpoint`
- `velodb_private_link_endpoint_service`

## Data sources

- `velodb_warehouses`
- `velodb_clusters`
- `velodb_warehouse_connections`
- `velodb_warehouse_versions`
- `velodb_private_link_endpoint_services`

## Known limitations

- SaaS warehouses can be created, updated, upgraded, rotated, and deleted.
- Existing BYOC warehouses can be imported and read; new BYOC warehouse creation
  is blocked by the provider.
- `velodb_cluster` manages `COMPUTE` clusters only.
- CPU and cache resize are applied one dimension at a time.
- The current Management API does not accept `maintenance_window`,
  `upgrade_policy`, mixed-billing request fields, or legacy
  `advanced_settings` in Terraform create/update requests.
