---
page_title: "velodb_warehouse Resource - velodb"
subcategory: ""
description: |-
  Manages a VeloDB Cloud warehouse with initial cluster provisioning, password management, and version upgrades.
---

# velodb_warehouse (Resource)

Manages a VeloDB Cloud warehouse.

For `deployment_mode = "SaaS"`, the resource creates, updates, upgrades,
rotates the admin password for, and deletes warehouses. For
`deployment_mode = "BYOC"`, the resource imports and reads existing warehouses;
new BYOC warehouse creation is intentionally blocked by the provider.

## Example Usage

### SaaS Warehouse

```terraform
resource "velodb_warehouse" "analytics" {
  name            = "analytics-saas"
  deployment_mode = "SaaS"
  cloud_provider  = "aws"
  region          = "us-east-1"

  admin_password = var.admin_password

  initial_cluster {
    zone         = "us-east-1a"
    compute_vcpu = 4
    cache_gb     = 100

    auto_pause {
      enabled              = true
      idle_timeout_minutes = 30
    }
  }

  timeouts {
    create = "30m"
    delete = "20m"
  }
}
```

### Imported BYOC Warehouse

```terraform
import {
  to = velodb_warehouse.production
  id = "AWVA7PYB"
}

resource "velodb_warehouse" "production" {
  name            = "test_cli"
  deployment_mode = "BYOC"
  cloud_provider  = "aws"
  region          = "us-east-1"
}

```

Then run:

```shell
terraform plan
terraform apply
```

BYOC warehouses must already exist before import. The provider will read fields such as `status`, `zone`, `core_version`, `initial_cluster_id`, and `byoc_setup` when the API returns them.

## Password Rotation

To rotate the warehouse admin password, change `admin_password` and apply. The
provider detects the sensitive value change and calls the password-change API.

```terraform
resource "velodb_warehouse" "example" {
  # ...
  admin_password = var.new_admin_password
}
```

`admin_password_version` remains in the schema for backward compatibility, but
it is not required for password rotation.

## Version Upgrade

The warehouse upgrade API now requires a numeric `targetVersionId` instead of a version string. Use the `velodb_warehouse_versions` data source to discover valid IDs and pass one as `core_version_id`:

```terraform
data "velodb_warehouse_versions" "available" {
  warehouse_id = velodb_warehouse.example.id
}

resource "velodb_warehouse" "example" {
  # ...
  core_version_id = data.velodb_warehouse_versions.available.default_id
  # or pin to a specific version_id from data.velodb_warehouse_versions.available.versions
}
```

The provider calls the upgrade API and waits for completion when
`core_version_id` changes. The `core_version` string attribute is read-only.

## Managing the Initial Cluster

The VeloDB API requires an `initial_cluster` block at SaaS warehouse creation.
The `initial_cluster` block is create-only. To manage or delete the initial
cluster later, import it into a separate `velodb_cluster` resource.

The warehouse exposes `initial_cluster_id` as a computed output to simplify this workflow:

```terraform
resource "velodb_warehouse" "main" {
  name            = "analytics"
  deployment_mode = "SaaS"
  cloud_provider  = "aws"
  region          = "us-east-1"
  admin_password  = var.admin_password

  initial_cluster {
    zone         = "us-east-1a"
    compute_vcpu = 4
    cache_gb     = 100

    auto_pause {
      enabled              = true
      idle_timeout_minutes = 30
    }
  }
}

# Add a second cluster before deleting the initial cluster.
resource "velodb_cluster" "etl" {
  warehouse_id = velodb_warehouse.main.id
  name         = "etl"
  cluster_type = "COMPUTE"
  zone         = "us-east-1a"
  compute_vcpu = 16
  cache_gb     = 400
}

# Import the initial cluster so it becomes a first-class managed resource
import {
  to = velodb_cluster.initial
  id = "${velodb_warehouse.main.id}/${velodb_warehouse.main.initial_cluster_id}"
}

resource "velodb_cluster" "initial" {
  warehouse_id = velodb_warehouse.main.id
  name         = "bootstrap"
  cluster_type = "COMPUTE"
  zone         = "us-east-1a"
  compute_vcpu = 4
  cache_gb     = 100
}
```

To destroy the initial cluster later:

1. Confirm the warehouse has at least one other cluster (e.g. `velodb_cluster.etl` in the example above).
2. Remove both the `resource "velodb_cluster" "initial" { ... }` block and the `import {}` block from your configuration.
3. Run `terraform apply`.

## Known Limitations

- BYOC warehouses can be imported and read, but this provider does not create
  new BYOC warehouses. Attempting to create `deployment_mode = "BYOC"` returns
  `BYOC warehouse creation is not supported`.
- The current Management API does not expose `maintenance_window`,
  `upgrade_policy`, or legacy `advanced_settings` on warehouse create/update.
- The warehouse's last cluster cannot be deleted. Add another cluster first, or
  destroy the whole warehouse.
- Prepaid clusters cannot be deleted until they expire. This is an API billing
  constraint.
- `admin_password` is write-only in the API and is stored in Terraform state as
  a sensitive value so Terraform can detect password rotation.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `cloud_provider` (String) Cloud provider for the warehouse (e.g., `aws`, `aliyun`). Changing this forces a new resource.
- `deployment_mode` (String) Deployment mode: `BYOC` or `SaaS`. Changing this forces a new resource.
- `name` (String) Warehouse display name.
- `region` (String) Cloud region (e.g., `us-east-1`, `cn-beijing`). Changing this forces a new resource.

### Optional

- `admin_password` (String, Sensitive) Administrator password. Set on creation and used for password rotation. The password is stored in state since it cannot be read back from the API.
- `admin_password_version` (Number) Increment this value to trigger a password change. Must be used together with `admin_password`.
- `core_version_id` (Number) Target engine version ID. Changing this triggers a warehouse upgrade. Discover valid values via the `velodb_warehouse_versions` data source.
- `setup_mode` (String) BYOC setup mode: `guided` or `advanced`. BYOC creation is blocked by this provider; this attribute remains for API compatibility. Changing this forces a new resource.
- `credential_id` (Number) Credential identifier for Wizard mode. Changing this forces a new resource.
- `initial_cluster` (Block List, Max: 1) Initial cluster created together with the warehouse. This is a create-only configuration. After creation, manage the cluster lifecycle by importing it as a `velodb_cluster` resource. (see [below for nested schema](#nestedblock--initial_cluster))
- `network_config_id` (Number) Existing network configuration identifier for Wizard mode. Changing this forces a new resource.
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `vpc_mode` (String) VPC consistency hint for Template mode: `existing` or `new`. Changing this forces a new resource.

### Read-Only

- `byoc_setup` (Block List) BYOC setup guidance returned for BYOC warehouses. (see [below for nested schema](#nestedatt--byoc_setup))
- `core_version` (String) Current human-readable engine version reported by the API (e.g. `3.0.8`). Read-only. Set `core_version_id` to trigger upgrades.
- `created_at` (String) Warehouse creation time in ISO 8601 / RFC 3339 format.
- `expire_time` (String) Warehouse expiration time when available.
- `id` (String) Warehouse identifier (e.g., `ALBJ07YE`).
- `initial_cluster_id` (String) ID of the initial cluster created with the warehouse. Use this with an `import {}` block to manage the initial cluster as a `velodb_cluster` resource. See [Managing the Initial Cluster](#managing-the-initial-cluster).
- `endpoint_service_id` (String) PrivateLink endpoint service ID when available.
- `endpoint_service_name` (String) PrivateLink endpoint service name when available.
- `pay_type` (String) Billing type: `PostPaid` or `PrePaid`.
- `status` (String) Current warehouse status. One of: `Creating`, `Running`, `Resizing`, `Adjusting`, `Upgrading`, `Suspending`, `Resuming`, `Stopping`, `Starting`, `Restarting`, `Deleting`, `Suspended`, `Stopped`, `Deleted`, `CreateFailed`.
- `zone` (String) Primary availability zone derived from the SQL cluster.

<a id="nestedblock--initial_cluster"></a>
### Nested Schema for `initial_cluster`

Required:

- `cache_gb` (Number) Cache capacity in GB.
- `compute_vcpu` (Number) Compute capacity in vCPUs.
- `zone` (String) Availability zone for the initial cluster.

Optional:

- `auto_pause` (Block List, Max: 1) Auto-pause configuration. (see [below for nested schema](#nestedblock--initial_cluster--auto_pause))

<a id="nestedblock--initial_cluster--auto_pause"></a>
### Nested Schema for `initial_cluster.auto_pause`

Required:

- `enabled` (Boolean) Whether auto-pause is enabled.

Optional:

- `idle_timeout_minutes` (Number) Idle timeout in minutes before the cluster can be paused automatically. Required when `enabled` is `true`.

<a id="nestedatt--byoc_setup"></a>
### Nested Schema for `byoc_setup`

Read-Only:

- `doc_url` (String) Documentation URL for the standard BYOC path.
- `doc_url_for_new_vpc` (String) Documentation URL for the new-VPC BYOC path.
- `shell_command` (String) Shell command for provider-side BYOC setup.
- `shell_command_for_new_vpc` (String) Shell command for the new-VPC setup path.
- `token` (String) Short-lived token used by the downstream BYOC setup flow.
- `url` (String) Guided setup URL for the standard BYOC path.
- `url_for_new_vpc` (String) Guided setup URL for the new-VPC BYOC path.

<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String) Timeout for warehouse creation. Default: `45m`.
- `delete` (String) Timeout for warehouse deletion. Default: `20m`.
- `update` (String) Timeout for warehouse updates (including upgrades). Default: `15m`.

## Import

Import is supported using the following syntax:

```shell
# Warehouses can be imported by specifying the warehouse ID.
terraform import velodb_warehouse.example ALBJ07YE
```

Or using the Terraform 1.5+ import block:

```terraform
import {
  to = velodb_warehouse.example
  id = "ALBJ07YE"
}
```

~> **Note:** The `admin_password`, `admin_password_version`, and `initial_cluster` attributes cannot be read from the API and will not be populated after import. For imported BYOC warehouses, omit those create-only fields unless you intend to rotate the password after import.
