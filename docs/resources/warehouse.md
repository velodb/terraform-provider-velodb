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

Use `core_version` for both creation and in-place upgrades:

```terraform
resource "velodb_warehouse" "production" {
  # ...
  core_version = "26.1" # creation: backend selects the latest patch in 26.1
}
```

Creation accepts `major.minor` or exact `major.minor.patch`. To upgrade from
`26.1.1` to `26.1.2`, set `core_version = "26.1.2"`. The provider resolves the exact
string through the warehouse's available upgrade versions and calls the existing
upgrade API; it does not replace the warehouse. Missing, ambiguous, or invalid
upgrade targets produce an error. Downgrades are rejected. A two-part value is a
creation selector, not an instruction to continuously upgrade to newer patches;
changing to another release line after creation requires a three-part target.

`core_version` is the only version field needed for the new workflow. A matching
two-part configuration stays two-part in state to keep plans stable; an exact
three-part configuration stays exact. When omitted, it reports the full API
version without managing upgrades. Before upgrading from a two-part selector,
the provider fetches the actual running patch internally for validation.

The deprecated `initial_core_version` input remains supported for compatibility;
use `core_version` for new configurations. The legacy `core_version_id` input also remains supported.
The former accepts two- or three-part versions for creation only; the latter
continues to accept an upgrade ID. Do not combine `core_version` with either
legacy input. To migrate, remove the legacy input and set `core_version` to the
current full version (no upgrade), or to an eligible newer three-part version.

## Public Access Policy

For `deployment_mode = "BYOC"`, you can set the initial public access policy at
creation with a `public_access_policy` block. It reuses the same policy values as the
`velodb_warehouse_public_access_policy` resource: `DENY_ALL`, `ALLOW_ALL`, or
`ALLOWLIST_ONLY` with CIDR `rules`.

```terraform
resource "velodb_warehouse" "production" {
  # ...
  deployment_mode = "BYOC"

  core_version = "26.1"

  public_access_policy {
    policy = "ALLOWLIST_ONLY"

    rules = [
      {
        cidr        = "203.0.113.0/24"
        description = "office"
      },
    ]
  }
}
```

The block sets the initial policy for BYOC warehouses and manages subsequent
changes in place using the policy update API. It can also be added to an existing
warehouse. The creation API still rejects an initial policy for SaaS warehouses.
Refresh reads the managed policy and rules from the API, so external changes
appear in the next plan, including externally cleared allowlists.

Removing the block stops managing the policy without changing remote access.
Set `policy = "DENY_ALL"` explicitly to disable public access.

Do not manage the same warehouse's policy with both this block and a
`velodb_warehouse_public_access_policy` resource. The standalone resource remains
supported for compatibility and policies managed separately from a warehouse.
When migrating from it, first remove its state ownership with `terraform state rm`
(or a `removed` block with `destroy = false`) and remove its configuration, then
add the matching inline block. Do not destroy the standalone resource as part of
that migration: its delete action resets the remote policy to `DENY_ALL`.
When upgrading older configurations that use both forms, choose one owner before
applying; the inline block now actively reconciles the policy.

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

- `cloud_provider` (String) Cloud provider for the warehouse. Only `aws` is supported. Changing this forces a new resource.
- `deployment_mode` (String) Deployment mode: `BYOC` or `SaaS`. Changing this forces a new resource.
- `name` (String) Warehouse display name.
- `region` (String) Cloud region (e.g., `us-east-1`). Changing this forces a new resource.

### Optional

- `public_access_policy` (Block List, Max: 1) Public access policy updated in place with drift detection. Initial provisioning supports BYOC only. Removing the block stops management without changing remote access. (see [below for nested schema](#nestedblock--public_access_policy))
- `admin_password` (String, Sensitive) Administrator password. Set on creation and used for password rotation. The password is stored in state since it cannot be read back from the API.
- `admin_password_version` (Number) Increment this value to trigger a password change. Must be used together with `admin_password`.
- `core_version` (String) Desired version. Creation accepts two or three numeric parts; upgrades require an exact three-part target. Omit to leave upgrades unmanaged.
- `core_version_id` (Number) Legacy target version ID for an in-place upgrade. Prefer `core_version`.
- `initial_core_version` (String, Deprecated) Legacy creation-only version selector, accepting two or three numeric parts. Prefer `core_version`.
- `setup_mode` (String) BYOC setup mode. Set to `advanced` for AWS custom-infrastructure creation. Guided/template setup is not supported. Changing this forces a new resource.
- `credential_id` (Number) Registered credential configuration ID for advanced AWS BYOC. Read from the API when available. Changes are rejected; omission retains the existing binding.
- `ebs_encryption_key_id` (Number) Registered encryption key ID used to encrypt the warehouse's EBS volumes. Create the key with `velodb_encryption_key` (`use_ebs = true`). Changing this forces a new resource.
- `initial_cluster` (Block List, Max: 1) Initial cluster created together with the warehouse. This is a create-only configuration. After creation, manage the cluster lifecycle by importing it as a `velodb_cluster` resource. (see [below for nested schema](#nestedblock--initial_cluster))
- `network_config_id` (Number) Registered network configuration ID for advanced AWS BYOC. Read from the API when available. Changes are rejected; omission retains the existing binding.
- `tde_encryption_key_id` (Number) Registered encryption key ID used for transparent data encryption (TDE) of warehouse data. Create the key with `velodb_encryption_key` (`use_tde = true`). Changing this forces a new resource.
- `tags` (Map of String) Warehouse tags as key/value pairs. Create-only: the management API accepts tags only at creation and does not return or update them, so changing tags after creation is rejected.
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `vpc_mode` (String) VPC consistency hint for Template mode: `existing` or `new`. Changing this forces a new resource.

### Read-Only

- `byoc_setup` (Block List) BYOC setup guidance returned for BYOC warehouses. (see [below for nested schema](#nestedatt--byoc_setup))
- `created_at` (String) Warehouse creation time in ISO 8601 / RFC 3339 format.
- `expire_time` (String) Warehouse expiration time when available.
- `id` (String) Warehouse identifier (e.g., `ALBJ07YE`).
- `initial_cluster_id` (String) ID of the initial cluster created with the warehouse. Use this with an `import {}` block to manage the initial cluster as a `velodb_cluster` resource. See [Managing the Initial Cluster](#managing-the-initial-cluster).
- `endpoint_service_id` (String) PrivateLink endpoint service ID when available.
- `endpoint_service_name` (String) PrivateLink endpoint service name when available.
- `pay_type` (String) Billing type: `PostPaid` or `PrePaid`.
- `status` (String) Current warehouse status. One of: `Creating`, `Running`, `Resizing`, `Adjusting`, `Upgrading`, `Suspending`, `Resuming`, `Stopping`, `Starting`, `Restarting`, `Deleting`, `Suspended`, `Stopped`, `Deleted`, `CreateFailed`.
- `zone` (String) Primary availability zone derived from the SQL cluster.

<a id="nestedblock--public_access_policy"></a>
### Nested Schema for `public_access_policy`

Required:

- `policy` (String) Public access policy: `DENY_ALL`, `ALLOW_ALL`, or `ALLOWLIST_ONLY`.

Optional:

- `rules` (Attributes Set) Allowlist CIDR rules. Only valid when `policy` is `ALLOWLIST_ONLY`. Order is not significant. (see [below for nested schema](#nestedatt--public_access_policy--rules))

<a id="nestedatt--public_access_policy--rules"></a>
### Nested Schema for `public_access_policy.rules`

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

~> **Note:** The `tde_encryption_key_id` and `ebs_encryption_key_id` attributes are populated from the API after import. `credential_id` and `network_config_id` are also recovered when the backend returns `credentialId` and `networkConfigId`. Older backends that omit these fields preserve known state values; after import, supply the existing IDs if needed. Missing fields cannot be used to detect association removal. The `admin_password`, `admin_password_version`, `initial_cluster`, `initial_core_version` attributes cannot be read from the API. Set those create-only fields in your configuration to match the existing warehouse before the next plan; the provider permits initialization of missing create-only bindings after import, but subsequent binding changes are rejected.

## Immutable infrastructure

Changes to deployment mode, cloud provider, region, setup mode, infrastructure
bindings, initial cluster zone, or configured encryption key IDs are rejected
after creation instead of scheduling warehouse replacement. Unknown binding
values are also rejected because they may represent an upstream replacement.
Restore the original configuration; use a separate warehouse for migration.
Optional computed encryption key IDs omitted from configuration retain their
API-reported values; omission does not disable encryption. BYOC modules also
reject changes to their recorded encryption creation flags and network inputs.
Explicit destruction remains supported.

API responses using the legacy `WHITELIST_ONLY` value are normalized to
`ALLOWLIST_ONLY` on refresh, preserving allowlist rules. Continue using
`ALLOWLIST_ONLY` in Terraform configuration; requests use the current enum name.
