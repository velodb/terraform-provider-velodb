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
`deployment_mode = "BYOC"`, it creates AWS warehouses with registered custom
infrastructure through the advanced setup flow. Existing warehouses can also be
imported.

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

### AWS BYOC Warehouse

```terraform
resource "velodb_byoc_credential" "production" {
  cloud_provider            = "aws"
  name                      = "production-credential"
  region                    = "us-east-1"
  bucket_name               = var.bucket_name
  data_credential_arn       = var.data_credential_arn
  deployment_credential_arn = var.deployment_credential_arn
}

resource "velodb_byoc_network" "production" {
  cloud_provider    = "aws"
  name              = "production-network"
  credential_id     = velodb_byoc_credential.production.id
  security_group_id = var.security_group_id

  zone_mappings = [{
    zone_id   = "us-east-1a"
    subnet_id = var.subnet_id
  }]
}

resource "velodb_warehouse" "production" {
  name              = "analytics-byoc"
  deployment_mode   = "BYOC"
  cloud_provider    = "aws"
  region            = "us-east-1"
  setup_mode        = "advanced"
  credential_id     = velodb_byoc_credential.production.id
  network_config_id = velodb_byoc_network.production.id
  admin_password    = var.admin_password

  initial_cluster {
    zone         = "us-east-1a"
    compute_vcpu = 4
    cache_gb     = 100
  }
}
```

Use three unique `zone_mappings` entries for a multi-AZ network. The VeloDB API
validates the IAM roles, bucket, subnet-to-zone relationship, security group,
and optional VPC endpoint during registration.

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

To pin the engine version at creation instead, set `version` to a
`major.minor` value (e.g. `26.1`). Only two-part versions are
accepted; three-part versions are rejected. The management API selects the
newest matching build for that line. `version` is create-only; use
`core_version_id` to upgrade afterward. `version` and `core_version_id` are
mutually exclusive — set one or the other, not both.

## Initial Access Policy

For `deployment_mode = "BYOC"`, you can set the initial public access policy at
creation with an `access_policy` block. It reuses the same policy values as the
`velodb_warehouse_public_access_policy` resource: `DENY_ALL`, `ALLOW_ALL`, or
`ALLOWLIST_ONLY` with CIDR `rules`.

```terraform
resource "velodb_warehouse" "production" {
  # ...
  deployment_mode = "BYOC"

  version = "26.1"

  access_policy {
    policy = "ALLOWLIST_ONLY"

    rules {
      cidr        = "203.0.113.0/24"
      description = "office"
    }
  }
}
```

`access_policy` is BYOC-only and create-only: it configures the policy once
during provisioning. SaaS warehouses reject `access_policy`.

~> **Note:** `access_policy` is never read back from the API, so Terraform does
not detect drift on it. If the policy is later changed out-of-band (via the
console or another tool), `terraform plan` will not report a difference, and the
state value remains what was applied at creation.

~> **Note:** Do not manage the same warehouse's public access policy with both
`access_policy` here and a separate `velodb_warehouse_public_access_policy`
resource — they write the same endpoint with no ordering guarantee, so the
result is non-deterministic. Use `access_policy` only to seed the initial policy,
then manage it exclusively with `velodb_warehouse_public_access_policy`
afterward. You need not remove the `access_policy` block, since it is create-only
and has no effect after provisioning.

## Managing the Initial Cluster

The VeloDB API requires an `initial_cluster` block at warehouse creation.
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

- New BYOC warehouse creation supports AWS custom infrastructure with
  `setup_mode = "advanced"`. Guided/template setup and other cloud providers are
  not supported yet.
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

- `access_policy` (Block List, Max: 1) Initial public access policy applied at warehouse creation. BYOC-only and create-only. After creation, manage the policy with the `velodb_warehouse_public_access_policy` resource. (see [below for nested schema](#nestedblock--access_policy))
- `admin_password` (String, Sensitive) Administrator password. Set on creation and used for password rotation. The password is stored in state since it cannot be read back from the API.
- `admin_password_version` (Number) Increment this value to trigger a password change. Must be used together with `admin_password`.
- `core_version_id` (Number) Target engine version ID. Changing this triggers a warehouse upgrade. Discover valid values via the `velodb_warehouse_versions` data source.
- `version` (String) Initial engine version to provision, in `major.minor` numeric format (e.g. `26.1`). Two-part only; three-part versions are rejected. The management API selects the newest matching build for that line. Create-only; use `core_version_id` to upgrade an existing warehouse.
- `setup_mode` (String) BYOC setup mode. Set to `advanced` for AWS custom-infrastructure creation. Guided/template setup is not supported. Changing this forces a new resource.
- `credential_id` (Number) Registered credential configuration ID for advanced AWS BYOC. Changing this forces a new resource.
- `initial_cluster` (Block List, Max: 1) Initial cluster created together with the warehouse. This is a create-only configuration. After creation, manage the cluster lifecycle by importing it as a `velodb_cluster` resource. (see [below for nested schema](#nestedblock--initial_cluster))
- `network_config_id` (Number) Registered network configuration ID for advanced AWS BYOC. Changing this forces a new resource.
- `tags` (Map of String) Warehouse tags as key/value pairs. Create-only: the management API accepts tags only at creation and does not return or update them, so changing tags after creation is rejected.
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `vpc_mode` (String) VPC consistency hint for Template mode: `existing` or `new`. Changing this forces a new resource.

### Read-Only

- `byoc_setup` (Block List) BYOC setup guidance returned for BYOC warehouses. (see [below for nested schema](#nestedatt--byoc_setup))
- `core_version` (String) Current human-readable engine version reported by the API (e.g. `26.1.0`). Read-only. Set `core_version_id` to trigger upgrades.
- `created_at` (String) Warehouse creation time in ISO 8601 / RFC 3339 format.
- `expire_time` (String) Warehouse expiration time when available.
- `id` (String) Warehouse identifier (e.g., `ALBJ07YE`).
- `initial_cluster_id` (String) ID of the initial cluster created with the warehouse. Use this with an `import {}` block to manage the initial cluster as a `velodb_cluster` resource. See [Managing the Initial Cluster](#managing-the-initial-cluster).
- `endpoint_service_id` (String) PrivateLink endpoint service ID when available.
- `endpoint_service_name` (String) PrivateLink endpoint service name when available.
- `pay_type` (String) Billing type: `PostPaid` or `PrePaid`.
- `status` (String) Current warehouse status. One of: `Creating`, `Running`, `Resizing`, `Adjusting`, `Upgrading`, `Suspending`, `Resuming`, `Stopping`, `Starting`, `Restarting`, `Deleting`, `Suspended`, `Stopped`, `Deleted`, `CreateFailed`.
- `zone` (String) Primary availability zone derived from the SQL cluster.

<a id="nestedblock--access_policy"></a>
### Nested Schema for `access_policy`

Required:

- `policy` (String) Public access policy: `DENY_ALL`, `ALLOW_ALL`, or `ALLOWLIST_ONLY`.

Optional:

- `rules` (Attributes Set) Allowlist CIDR rules. Only valid when `policy` is `ALLOWLIST_ONLY`. Order is not significant. (see [below for nested schema](#nestedatt--access_policy--rules))

<a id="nestedatt--access_policy--rules"></a>
### Nested Schema for `access_policy.rules`

Required:

- `cidr` (String) CIDR block or single IP.

Optional:

- `description` (String) Optional rule description.

<a id="nestedblock--initial_cluster"></a>
### Nested Schema for `initial_cluster`

Required:

- `cache_gb` (Number) Cache capacity in GB.
- `compute_vcpu` (Number) Compute capacity in vCPUs.
- `zone` (String) Availability zone for the initial cluster.

Optional:

- `auto_pause` (Block List, Max: 1) Auto-pause configuration. (see [below for nested schema](#nestedblock--initial_cluster--auto_pause))
- `ratio` (Number) vCPU-to-memory ratio. Supported values are `4` (1:4) and `8` (1:8). This value is create-only. If omitted, Manager defaults to `8` (1:8).

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

~> **Note:** The `admin_password`, `admin_password_version`, `initial_cluster`, `version`, and `access_policy` attributes cannot be read from the API and will not be populated after import. Omit those create-only fields unless you intend to rotate the password after import.
