---
page_title: "velodb_warehouse Resource - velodb"
subcategory: ""
description: |-
  Manages a VeloDB Cloud warehouse with initial cluster provisioning, password management, and version upgrades.
---

# velodb_warehouse (Resource)

Manages a VeloDB Cloud warehouse. `deployment_mode` must be `SaaS` or `BYOC`.

A warehouse is the primary unit of deployment. It belongs to an organization, runs on a specific cloud provider and region, and contains one or more clusters. The resource manages the full warehouse lifecycle including creation, updates, version upgrades, password rotation, and deletion.

Key capabilities:

- **SaaS and BYOC** deployment modes
- **Initial cluster** created atomically with the warehouse
- **Password rotation** — change `admin_password` and apply (no version bump needed)
- **Version upgrades** triggered declaratively by changing `core_version_id` (lookup IDs via the [`velodb_warehouse_versions`](../data-sources/warehouse_versions.md) data source)
- **BYOC setup guidance** (shell commands, template URLs) exposed as computed `byoc_setup` block

## Supported / not supported features

| Feature | Status | Notes |
|---|---|---|
| SaaS warehouse | ✅ Supported | `deployment_mode = "SaaS"` |
| BYOC `guided` mode | ⚠️ API works but not IaC-friendly | Returns CFN template URL; customer must click-through in AWS console to finish. Terraform apply completes, but warehouse stays in `Creating` until CFN runs. |
| BYOC `advanced` mode | ❌ Not supported by sandbox API | Spec documents it, but `POST /v1/warehouses` with `setupMode=advanced` returns `400 InvalidParameter` in the current sandbox. Provider code is correct per spec — awaiting API fix. |
| Delete stuck BYOC `Creating` warehouse | ❌ Not supported by API | If guided-mode CFN is never executed, `DELETE` returns 500 "unfinished operations". Requires VeloDB admin intervention. |
| Password rotation | ✅ Supported | Change `admin_password` — provider calls `POST /settings/password` automatically |
| Version upgrade | ✅ Supported | Change `core_version_id` (int64) — provider calls `POST /settings/upgrade` and polls for completion. Use the `velodb_warehouse_versions` data source to discover valid IDs. |
| Maintenance window / upgrade policy | ❌ Not in current API | The current management API does not expose these fields on warehouse create/update. |
| Advanced settings update | ❌ Removed in API | The previous `/settings` `advancedSettings` field was dropped by the upstream Management API. No replacement at the resource level. |
| Delete initial cluster | ✅ Supported | Import via `initial_cluster_id` and manage as `velodb_cluster`. See [Managing the Initial Cluster](#managing-the-initial-cluster). |
| Delete warehouse with pre-paid clusters | ❌ Not supported until clusters expire | API billing semantics |

## Example Usage

### SaaS Warehouse

```terraform
resource "velodb_warehouse" "analytics" {
  name            = "analytics-saas"
  deployment_mode = "SaaS"
  cloud_provider  = "aliyun"
  region          = "cn-beijing"

  admin_password         = var.admin_password

  initial_cluster {
    zone         = "cn-beijing-k"
    compute_vcpu = 4
    cache_gb     = 1000

    auto_pause {
      enabled              = false
      idle_timeout_minutes = 30
    }
  }

  timeouts {
    create = "30m"
    delete = "20m"
  }
}
```

### BYOC Warehouse with Guided Mode (CloudFormation)

```terraform
resource "velodb_warehouse" "production" {
  name            = "production-byoc"
  deployment_mode = "BYOC"
  cloud_provider  = "aliyun"
  region          = "cn-beijing"
  setup_mode     = "guided"
  vpc_mode        = "existing"

  admin_password = var.admin_password

  initial_cluster {
    zone         = "cn-beijing-k"
    compute_vcpu = 8
    cache_gb     = 400

    auto_pause {
      enabled              = true
      idle_timeout_minutes = 30
    }
  }

  timeouts {
    create = "45m"
    delete = "20m"
  }
}

# Use the BYOC setup shell command to provision cloud resources
output "byoc_shell_command" {
  value     = velodb_warehouse.production.byoc_setup[0].shell_command
  sensitive = true
}
```

### BYOC Warehouse with Advanced Mode (AWS)

```terraform
resource "velodb_warehouse" "aws_byoc" {
  name            = "aws-byoc-wizard"
  deployment_mode = "BYOC"
  cloud_provider  = "aws"
  region          = "us-east-1"
  setup_mode     = "advanced"

  credential_id            = 12345
  network_config_id        = 67890

  admin_password         = var.admin_password

  initial_cluster {
    zone         = "us-east-1a"
    compute_vcpu = 16
    cache_gb     = 800
  }

  timeouts {
    create = "45m"
  }
}
```

### Password Rotation

To rotate the warehouse admin password, just change `admin_password`. The provider detects the value change and calls the password-change API.

```terraform
resource "velodb_warehouse" "example" {
  # ...
  admin_password = var.new_admin_password  # change to rotate
}
```

~> The `admin_password_version` attribute exists on the schema for backwards compatibility but is **not required** to trigger rotation. The provider detects changes to `admin_password` directly.

### Version Upgrade

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

The provider will call the upgrade API and wait for completion when `core_version_id` changes.

~> **Migration note:** The previous `core_version` (string) attribute is now read-only. Configurations that set `core_version = "3.1.0"` must switch to `core_version_id` referencing a `version_id` from the `velodb_warehouse_versions` data source. The string-based upgrade endpoint returns `400 InvalidParameter — targetVersionId is required` in the new API.

### Managing the Initial Cluster

The VeloDB API requires an `initial_cluster` block at warehouse creation — a warehouse cannot exist without at least one cluster. The `initial_cluster` block is **create-only** (changes to it after creation are ignored). To manage or delete the initial cluster later (resize, pause, destroy), import it into a separate `velodb_cluster` resource.

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

# Add a second cluster (required before the initial cluster can be deleted)
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

To **destroy the initial cluster later**:

1. Confirm the warehouse has at least one other cluster (e.g. `velodb_cluster.etl` in the example above).
2. Remove both the `resource "velodb_cluster" "initial" { ... }` block and the `import {}` block from your configuration.
3. `terraform apply` — the initial cluster is deleted via the API.

~> **Important constraints on initial cluster deletion:**
> - The warehouse's **last cluster cannot be deleted** — add another cluster first, or destroy the whole warehouse.
> - **Prepaid (subscription) clusters cannot be deleted until they expire** — this is an API billing constraint, not a Terraform limitation.
> - The initial cluster is created with the API default billing model, so it's normally deletable when it is not prepaid.

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
- `setup_mode` (String) BYOC creation mode: `Template` or `Wizard`. `Wizard` is only supported for `aws`. Changing this forces a new resource.
- `credential_id` (Number) Credential identifier for Wizard mode. Changing this forces a new resource.
- `initial_cluster` (Block List, Max: 1) Initial cluster created together with the warehouse. This is a create-only configuration — after creation, manage the cluster lifecycle by importing it as a `velodb_cluster` resource. (see [below for nested schema](#nestedblock--initial_cluster))
- `network_config_id` (Number) Existing network configuration identifier for Wizard mode. Changing this forces a new resource.
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `vpc_mode` (String) VPC consistency hint for Template mode: `existing` or `new`. Changing this forces a new resource.

### Read-Only

- `byoc_setup` (Block List) BYOC setup guidance returned for BYOC warehouses. (see [below for nested schema](#nestedatt--byoc_setup))
- `core_version` (String) Current human-readable engine version reported by the API (e.g. `3.0.8`). Read-only — set `core_version_id` to trigger upgrades.
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

- `idle_timeout_minutes` (Number) Idle timeout in minutes before the cluster can be paused automatically.

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

~> **Note:** The `admin_password`, `admin_password_version`, `initial_cluster`, and `advanced_settings` attributes cannot be read from the API and will not be populated after import. You must add them to your configuration manually.
