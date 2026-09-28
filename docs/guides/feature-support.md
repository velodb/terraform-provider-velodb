# Warehouse feature support

What each `velodb_warehouse` feature does at **creation** and **update** time,
which **deployment modes** (SaaS / BYOC) support it, and whether the BYOC
Terraform modules (`modules/aws_byoc`, `modules/aws_byoc_existing`) expose it.
Behavior is defined by the VeloDB Cloud Management API (`formation/api`); this
table tracks the Terraform surface over it.

A `velodb_warehouse` is either **SaaS** (VeloDB-hosted) or **BYOC** (deployed in
your own cloud account), selected by `deployment_mode`. Most features apply to
both; some are BYOC-only. The Terraform **modules** provision BYOC warehouses
only — SaaS warehouses use the `velodb_warehouse` resource directly.

**Update legend**

- ✅ **In-place** — changed on the existing warehouse.
- ♻️ **Replaces** — changing it destroys and recreates the warehouse.
- 🚫 **Create-only** — set once at creation; a later change is rejected.
- — **N/A** — read-only, or applied through a different endpoint/resource.

**Deployment-mode legend**

- **Both** — supported for SaaS and BYOC warehouses.
- **BYOC** — BYOC warehouses only.
- **SaaS** — SaaS warehouses only.

**Modules legend**

- ✅ — module variable of the same name.
- `auto` — set by the module (not a variable).
- `derived` — the `aws_byoc` module computes it; `aws_byoc_existing` takes it as input.
- ❌ — not exposed by the modules.

## Core

| Attribute | Creation | Update | Modes | Modules |
|---|---|---|---|---|
| `name` | ✅ | ✅ In-place | Both | `auto` (from `name_prefix`) |
| `deployment_mode` | ✅ | ♻️ Replaces | Both | `auto` (`BYOC`) |
| `cloud_provider` | ✅ | ♻️ Replaces | Both¹ | `auto` (`aws`) |
| `region` | ✅ | ♻️ Replaces | Both | ✅ `region` |
| `setup_mode` | ✅ | ♻️ Replaces | BYOC | `auto` (`advanced`) |
| `vpc_mode` | ✅ | ♻️ Replaces | BYOC | `auto` |
| `admin_password` | ✅ | ✅ In-place (bump `admin_password_version`) | Both | ✅ `admin_password` |
| `tags` | ✅ | 🚫 Create-only | Both | ✅ `tags` |

¹ Advanced BYOC (what the modules use) requires `aws`. VeloDB Cloud SaaS also
runs on Azure, but this Terraform provider has only been tested with `aws`.

## Core version

| Attribute | Creation | Update | Modes | Modules |
|---|---|---|---|---|
| `core_version` | ✅ Two or three parts | ✅ Three-part in-place upgrade | Both | ✅ `core_version` |
| `core_version_id` | Legacy post-create upgrade | ✅ Legacy ID upgrade | Both | ❌ |

Use `core_version = "26.1"` or `"26.1.1"` at creation. A two-part value lets the
backend choose the latest patch. Upgrade by setting an exact target such as
`"26.1.2"`; the provider resolves its eligible ID internally. Do not combine
`core_version` with `core_version_id`.

## Network & access

| Attribute | Creation | Update | Modes | Modules |
|---|---|---|---|---|
| `public_access_policy` | ✅ | 🚫 Create-only¹ | BYOC² | ✅ `public_access_policy` |
| `enable_tls` | ✅ | 🚫 Create-only | Both | ❌ |
| `enable_https` | ✅ | 🚫 Create-only | Both | ❌ |

¹ Change it after creation with the separate `velodb_warehouse_public_access_policy`
resource. `public_access_policy.rules` is a list — write `rules = [ { cidr = "…" } ]`.

² The management API rejects an initial `public_access_policy` for SaaS warehouses.

## BYOC infrastructure

In advanced BYOC (what the modules use) the warehouse references a credential
and network config; the S3 bucket, subnets, security group, and endpoint are
configured on `velodb_byoc_credential` / `velodb_byoc_network` (the modules
create them in `aws_byoc`, or take them as inputs in `aws_byoc_existing`).

All features in this section are **BYOC only** — SaaS warehouses are hosted by
VeloDB and do not take a credential, network config, or customer-managed keys.

| Attribute | Creation | Update | Modes | Modules |
|---|---|---|---|---|
| `credential_id` | ✅ | ♻️ Replaces | BYOC | `derived` |
| `network_config_id` | ✅ | ♻️ Replaces | BYOC | `derived` |
| `tde_encryption_key_id` | ✅ | ♻️ Replaces | BYOC | ✅ `create_tde_encryption_key` / `tde_kms_key_arn` |
| `ebs_encryption_key_id` | ✅ | ♻️ Replaces | BYOC | ✅ `create_ebs_encryption_key` / `ebs_kms_key_arn` |

## Initial cluster

Set on the warehouse at creation; resize or add clusters afterward with the
`velodb_cluster` resource (modules expose `additional_clusters`).

| Attribute | Creation | Update | Modes | Modules |
|---|---|---|---|---|
| `initial_cluster.compute_vcpu` | ✅ | — (via `velodb_cluster`) | Both | ✅ `compute_vcpu` |
| `initial_cluster.cache_gb` | ✅ | — (via `velodb_cluster`) | Both | ✅ `cache_gb` |
| `initial_cluster.auto_pause` | ✅ | — (via `velodb_cluster`) | Both | ✅ `auto_pause` |
| `initial_cluster.ratio` | ✅ | — (via `velodb_cluster`) | Both | ❌ (modules use explicit sizes) |
| additional clusters | — | ✅ `velodb_cluster` | Both | ✅ `additional_clusters` (incl. `auto_pause`) |

## API features not yet exposed in the provider

The Management API supports these; the Terraform provider (and therefore the
modules) does not yet surface them. Listed so parity gaps stay visible.

| API feature | Endpoint | Notes |
|---|---|---|
| `upgradePolicy` (automatic/manual) | warehouse settings update | Auto-upgrade policy. |
| `maintenanceWindow` (start/end hour UTC) | warehouse settings update | Upgrade maintenance window. |
| `crashReportEnabled` | crash-report status update | Toggle crash reporting. |
| `initialCluster.billingModel` / `period` / `periodUnit` | warehouse create | Reserved/committed billing for the initial cluster. |
