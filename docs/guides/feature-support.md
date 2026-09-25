# Warehouse feature support

What each `velodb_warehouse` feature does at **creation** and **update** time,
and whether the BYOC Terraform modules (`modules/aws_byoc`,
`modules/aws_byoc_existing`) expose it. Behavior is defined by the VeloDB Cloud
Management API (`formation/api`); this table tracks the Terraform surface over it.

**Update legend**

- ✅ **In-place** — changed on the existing warehouse.
- ♻️ **Replaces** — changing it destroys and recreates the warehouse.
- 🚫 **Create-only** — set once at creation; a later change is rejected.
- — **N/A** — read-only, or applied through a different endpoint/resource.

**Modules legend**

- ✅ — module variable of the same name.
- `auto` — set by the module (not a variable).
- `derived` — the `aws_byoc` module computes it; `aws_byoc_existing` takes it as input.
- ❌ — not exposed by the modules.

## Core

| Attribute | Creation | Update | Modules |
|---|---|---|---|
| `name` | ✅ | ✅ In-place | `auto` (from `name_prefix`) |
| `deployment_mode` | ✅ | ♻️ Replaces | `auto` (`BYOC`) |
| `cloud_provider` | ✅ | ♻️ Replaces | `auto` (`aws`) |
| `region` | ✅ | ♻️ Replaces | ✅ `region` |
| `setup_mode` | ✅ | ♻️ Replaces | `auto` (`advanced`) |
| `vpc_mode` | ✅ | ♻️ Replaces | `auto` |
| `admin_password` | ✅ | ✅ In-place (bump `admin_password_version`) | ✅ `admin_password` |
| `tags` | ✅ | 🚫 Create-only | ✅ `tags` |

## Core version

| Attribute | Creation | Update | Modules |
|---|---|---|---|
| `initial_core_version` | ✅ | 🚫 Create-only | ✅ `initial_core_version` |
| `core_version_id` | — | ✅ In-place (upgrade) | ❌ |
| `core_version` | — (read-only) | — | — |

Pin the initial `major.minor` with `initial_core_version`; upgrade later by setting
`core_version_id`. The two are mutually exclusive.

## Network & access

| Attribute | Creation | Update | Modules |
|---|---|---|---|
| `public_access_policy` | ✅ | 🚫 Create-only¹ | ✅ `public_access_policy` |
| `enable_tls` | ✅ | 🚫 Create-only | ❌ |
| `enable_https` | ✅ | 🚫 Create-only | ❌ |

¹ Change it after creation with the separate `velodb_warehouse_public_access_policy`
resource. `public_access_policy.rules` is a list — write `rules = [ { cidr = "…" } ]`.

## BYOC infrastructure

In advanced BYOC (what the modules use) the warehouse references a credential
and network config; the S3 bucket, subnets, security group, and endpoint are
configured on `velodb_byoc_credential` / `velodb_byoc_network` (the modules
create them in `aws_byoc`, or take them as inputs in `aws_byoc_existing`).

| Attribute | Creation | Update | Modules |
|---|---|---|---|
| `credential_id` | ✅ | ♻️ Replaces | `derived` |
| `network_config_id` | ✅ | ♻️ Replaces | `derived` |
| `tde_encryption_key_id` | ✅ | 🚫 Create-only | ❌ |
| `ebs_encryption_key_id` | ✅ | 🚫 Create-only | ❌ |

## Initial cluster

Set on the warehouse at creation; resize or add clusters afterward with the
`velodb_cluster` resource (modules expose `additional_clusters`).

| Attribute | Creation | Update | Modules |
|---|---|---|---|
| `initial_cluster.compute_vcpu` | ✅ | — (via `velodb_cluster`) | ✅ `compute_vcpu` |
| `initial_cluster.cache_gb` | ✅ | — (via `velodb_cluster`) | ✅ `cache_gb` |
| `initial_cluster.auto_pause` | ✅ | — (via `velodb_cluster`) | ✅ `auto_pause` |
| `initial_cluster.ratio` | ✅ | — (via `velodb_cluster`) | ❌ (modules use explicit sizes) |
| additional clusters | — | ✅ `velodb_cluster` | ✅ `additional_clusters` (incl. `auto_pause`) |

## API features not yet exposed in the provider

The Management API supports these; the Terraform provider (and therefore the
modules) does not yet surface them. Listed so parity gaps stay visible.

| API feature | Endpoint | Notes |
|---|---|---|
| `upgradePolicy` (automatic/manual) | warehouse settings update | Auto-upgrade policy. |
| `maintenanceWindow` (start/end hour UTC) | warehouse settings update | Upgrade maintenance window. |
| `crashReportEnabled` | crash-report status update | Toggle crash reporting. |
| `initialCluster.billingModel` / `period` / `periodUnit` | warehouse create | Reserved/committed billing for the initial cluster. |
