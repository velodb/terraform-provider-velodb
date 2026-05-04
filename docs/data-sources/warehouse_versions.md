# Data Source `velodb_warehouse_versions`

Lists valid upgrade target versions for a warehouse. Each entry has a numeric `version_id` that can be passed as `core_version_id` on the `velodb_warehouse` resource to trigger an upgrade.

The new Management API requires upgrades to reference a `targetVersionId` (int64) instead of a version string, so this data source is the source of truth for which versions are upgrade-eligible.

## Example

```terraform
data "velodb_warehouse_versions" "available" {
  warehouse_id = velodb_warehouse.main.id
}

# Pin to the API-recommended default
resource "velodb_warehouse" "main" {
  # ...
  core_version_id = data.velodb_warehouse_versions.available.default_id
}

# Or pick a specific version
output "all_versions" {
  value = data.velodb_warehouse_versions.available.versions
}
```

## Schema

### Required

- `warehouse_id` (String) Warehouse identifier.

### Read-Only

- `default_id` (Number) `version_id` of the default upgrade target, or `0` if none is marked default.
- `versions` (List of Object) All valid target versions returned by the API. Each element has:
  - `version_id` (Number) Engine version ID — pass this as `core_version_id`.
  - `version` (String) Human-readable version (e.g. `3.0.8`).
  - `description` (String) Version description or release label.
  - `is_default` (Bool) Whether this is the default upgrade target.

## Notes

The API may return an empty list if the warehouse has no upgrade targets currently available (already on the latest version, or upgrade-eligible window not yet open). Check the live response before assuming a version is available.
