# VeloDB Terraform Provider

This provider is aligned to the current VeloDB Management API YAML.

## Provider Configuration

```terraform
provider "velodb" {
  host    = "sandbox-api.velodb.io"
  api_key = var.velodb_api_key
}
```

`host` can also be set by `VELODB_HOST`; `api_key` can also be set by
`VELODB_API_KEY`.

## Supported Resources

- `velodb_warehouse`
- `velodb_cluster`
- `velodb_public_access_policy`
- `velodb_warehouse_private_endpoint`
- `velodb_private_link_endpoint_service`

## Supported Data Sources

- `velodb_warehouses`
- `velodb_clusters`
- `velodb_warehouse_connections`
- `velodb_warehouse_versions`

## Current API Notes

- Warehouse `deployment_mode` is `SaaS` or `BYOC`.
- Warehouse create/update does not accept `maintenance_window`,
  `upgrade_policy`, or legacy `advanced_settings`.
- Cluster create/update does not accept mixed-billing request fields. Terraform
  exposes `billing_model` as observed data only.
- Cluster pause/resume/reboot use explicit operation endpoints:
  `/pause`, `/resume`, and `/reboot`.
- Connections come from `GET /v1/warehouses/{warehouseId}/connections` and
  include public endpoints, private endpoints, compute clusters, and observer
  groups.
- Warehouse private endpoint registration uses
  `POST /v1/private-link/warehouses/{warehouseId}/endpoints`.

See the resource and data-source pages under `docs/resources` and
`docs/data-sources` for examples.
