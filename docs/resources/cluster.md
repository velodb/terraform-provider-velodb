---
page_title: "velodb_cluster Resource - velodb"
subcategory: ""
description: |-
  Manages a VeloDB Cloud cluster within a warehouse. Supports pure on-demand, pure subscription, or mixed billing via the subscription{} and on_demand{} pool blocks.
---

# velodb_cluster (Resource)

Use the *velodb_cluster* resource to create and manage clusters within a VeloDB Cloud warehouse.

A cluster is a compute unit within a warehouse. Each warehouse can contain multiple clusters of different types (`SQL`, `COMPUTE`, `OBSERVER`). Clusters can be independently scaled, paused, and resumed.

Key capabilities:

- **Pool-based billing** — each cluster has one or both of `subscription{}` and `on_demand{}` blocks (mixed billing)
- **Declarative lifecycle** — `desired_state` = `running` or `paused` (instead of imperative start/stop commands)
- **Auto-pause** — configure idle timeout for automatic pause to save costs
- **Independent pool resizing** — change `compute_vcpu` / `cache_gb` within each pool block
- **Connection info** — public/private endpoints and ports exposed as computed attributes

## Supported / not supported features

| Feature | Status | Notes |
|---|---|---|
| Create pure `on_demand` cluster | ✅ Supported | |
| Create pure `subscription` cluster | ✅ Supported | Requires `period` and `period_unit` |
| Create mixed cluster (both blocks) | ✅ Supported | Creates `on_demand` primary, then adds `subscription` via PATCH |
| Resize pool `compute_vcpu` | ✅ Supported | API auto-scales pool's `cache_gb` proportionally (plan modifier handles this) |
| Resize pool `cache_gb` | ✅ Supported | CPU and cache cannot be changed in the same request — done sequentially |
| Add `subscription` pool to existing `on_demand` cluster | ✅ Supported | Uses PATCH with `billingModel=subscription`; requires cluster >= 16 vCPU |
| Add `on_demand` pool to existing `subscription` cluster | ✅ Supported | Uses PATCH with `billingModel=on_demand`; requires cluster >= 16 vCPU |
| Remove a pool (shrink to single-pool) | ✅ Supported | PATCH with `computeVcpu=0` — provider does this automatically |
| Change `subscription.period` in place | ✅ Supported | PATCH with new period |
| Change `subscription.period_unit` in place | ❌ Not supported by API | Forces resource replacement |
| Change `subscription.auto_renew` | ✅ Supported | |
| Pause / resume (`desired_state`) | ✅ Supported | Calls `/pause` or `/resume` |
| Reboot (`reboot_trigger`) | ✅ Supported | Bump integer to trigger `/reboot` |
| Delete last cluster in warehouse | ❌ Not supported by API | Delete the warehouse instead |
| Delete prepaid (subscription) cluster | ❌ Not supported until expiration | Billing semantics — customer already paid for the period |
| Manual subscription renewal | ❌ Not exposed in Terraform | Use `auto_renew = true` |

## Example Usage

### Pure on-demand cluster (pay-as-you-go)

```terraform
resource "velodb_cluster" "etl" {
  warehouse_id  = velodb_warehouse.main.id
  name          = "compute-etl"
  cluster_type  = "COMPUTE"
  zone          = "us-east-1a"
  desired_state = "running"

  on_demand {
    compute_vcpu = 8
    cache_gb     = 100
  }

  auto_pause {
    enabled              = true
    idle_timeout_minutes = 15
  }

  timeouts {
    create = "20m"
    update = "20m"
  }
}

output "etl_endpoint" {
  value = velodb_cluster.etl.connection_info[0].public_endpoint
}
```

### Pure subscription cluster (reserved capacity)

```terraform
resource "velodb_cluster" "prod" {
  warehouse_id  = velodb_warehouse.main.id
  name          = "sql-primary"
  cluster_type  = "SQL"
  zone          = "us-east-1a"
  desired_state = "running"

  subscription {
    compute_vcpu = 16
    cache_gb     = 100
    period       = 1
    period_unit  = "Month"       # "Month" or "Year"
    auto_renew   = true
  }
}
```

### Mixed billing cluster (reserved base + on-demand burst)

Use both pool blocks. The provider creates the cluster with one pool first, then adds the second pool via PATCH.

```terraform
resource "velodb_cluster" "mixed" {
  warehouse_id  = velodb_warehouse.main.id
  name          = "bursty"
  cluster_type  = "COMPUTE"
  zone          = "us-east-1a"

  # Reserved baseline
  subscription {
    compute_vcpu = 16
    cache_gb     = 100
    period       = 1
    period_unit  = "Month"
    auto_renew   = true
  }

  # On-demand burst capacity
  on_demand {
    compute_vcpu = 16
    cache_gb     = 100
  }

  auto_pause {
    enabled              = true
    idle_timeout_minutes = 15
  }
}

# Computed mixed-billing fields
output "is_mixed" {
  value = velodb_cluster.mixed.is_mixed_billing      # true
}
output "total_cpu" {
  value = velodb_cluster.mixed.total_cpu             # 32
}
output "pool_breakdown" {
  value = {
    on_demand    = velodb_cluster.mixed.on_demand_node_count
    subscription = velodb_cluster.mixed.subscription_node_count
    total_nodes  = velodb_cluster.mixed.node_count
  }
}
```

### Mixed billing lifecycle (adding/removing pools over time)

```terraform
# Day 1 — start with pure on-demand
resource "velodb_cluster" "c" {
  warehouse_id = velodb_warehouse.main.id
  name         = "c"
  cluster_type = "COMPUTE"
  on_demand { compute_vcpu = 16, cache_gb = 100 }
}

# Day 30 — add subscription pool (cluster must be >= 16 vCPU)
# Edit config to:
#   on_demand    { compute_vcpu = 16, cache_gb = 100 }
#   subscription { compute_vcpu = 16, cache_gb = 100, period = 1, period_unit = "Month" }
# terraform apply → PATCH billingModel=subscription

# Day 60 — scale the subscription side only
#   subscription { compute_vcpu = 32, cache_gb = 100, period = 1, period_unit = "Month" }
# on_demand stays at 16/100 → PATCH billingModel=subscription, computeVcpu=32

# Day 90 — remove on-demand pool (go back to pure subscription)
# Delete the on_demand {} block from config
# terraform apply → provider calls PATCH billingModel=on_demand, computeVcpu=0
```

### Pause / resume / reboot

```terraform
# Pause
resource "velodb_cluster" "c" {
  # ...
  desired_state = "paused"    # was "running" → calls POST /pause
}

# Resume
resource "velodb_cluster" "c" {
  # ...
  desired_state = "running"   # was "paused" → calls POST /resume
}

# Reboot (increment the trigger integer)
resource "velodb_cluster" "c" {
  # ...
  reboot_trigger = 1          # was 0 → calls POST /reboot
}
```

The provider checks the cluster's current status before calling pause/resume, so redundant applies are no-ops (no API error).

### Independent pool resize

```terraform
# Scale only the on_demand pool from 16 to 32 vCPU
resource "velodb_cluster" "mixed" {
  # ...
  subscription {
    compute_vcpu = 16    # unchanged
    cache_gb     = 100
    period       = 1
    period_unit  = "Month"
  }
  on_demand {
    compute_vcpu = 32    # was 16 → PATCH billingModel=on_demand
    cache_gb     = 100
  }
}
```

~> **Note on cache_gb auto-scaling:** When `compute_vcpu` changes, the API auto-scales the pool's `cache_gb` proportionally. The provider handles this with a plan modifier that marks `cache_gb` as "known after apply" during vcpu changes. If you want to resize cache independently, change `cache_gb` in a separate apply (not combined with a vcpu change — the API rejects mixed cpu+disk changes in one request).

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `cluster_type` (String) Cluster type: `SQL`, `COMPUTE`, or `OBSERVER`. Changing this forces a new resource.
- `name` (String) Cluster display name.
- `warehouse_id` (String) Parent warehouse identifier. Changing this forces a new resource.

### Required (at least one of)

- `on_demand` (Block List, Max: 1) On-demand billing pool. (see [below for nested schema](#nestedblock--on_demand))
- `subscription` (Block List, Max: 1) Subscription billing pool. (see [below for nested schema](#nestedblock--subscription))

### Optional

- `auto_pause` (Block List, Max: 1) Auto-pause configuration. (see [below for nested schema](#nestedblock--auto_pause))
- `desired_state` (String) Desired cluster state: `running` or `paused`. Changing triggers the corresponding action.
- `reboot_trigger` (Number) Increment to trigger a cluster reboot.
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `zone` (String) Availability zone. Changing this forces a new resource.

### Read-Only

- `cloud_provider` (String) Cloud provider inherited from warehouse.
- `connection_info` (Attributes List) Connection endpoints. (see [below for nested schema](#nestedatt--connection_info))
- `created_at` (String) Creation time in RFC 3339 format.
- `expire_time` (String) Expiration time when applicable.
- `id` (String) Cluster identifier.
- `is_mixed_billing` (Boolean) `true` when both pools are present.
- `node_count` (Number) Total nodes across all pools.
- `on_demand_node_count` (Number) Nodes in the on-demand pool.
- `region` (String) Cloud region inherited from warehouse.
- `started_at` (String) Start time in RFC 3339 format.
- `status` (String) Current observed cluster status. One of: `Creating`, `Running`, `Resizing`, `Adjusting`, `Upgrading`, `Suspending`, `Resuming`, `Stopping`, `Starting`, `Restarting`, `Deleting`, `Suspended`, `Stopped`, `Deleted`, `CreateFailed`.
- `subscription_node_count` (Number) Nodes in the subscription pool.
- `total_cpu` (Number) Total CPU across all pools.
- `total_disk_gb` (Number) Total disk GB across all pools.

<a id="nestedblock--subscription"></a>
### Nested Schema for `subscription`

Required:

- `cache_gb` (Number) Cache capacity in GB. Auto-scales when `compute_vcpu` changes.
- `compute_vcpu` (Number) vCPU capacity of the subscription pool.
- `period` (Number) Subscription period length.
- `period_unit` (String) `Month` or `Year`. **In-place changes not supported** — forces replacement.

Optional:

- `auto_renew` (Boolean) Auto-renew at expiration.

<a id="nestedblock--on_demand"></a>
### Nested Schema for `on_demand`

Required:

- `cache_gb` (Number) Cache capacity in GB. Auto-scales when `compute_vcpu` changes.
- `compute_vcpu` (Number) vCPU capacity of the on-demand pool.

<a id="nestedblock--auto_pause"></a>
### Nested Schema for `auto_pause`

Required:

- `enabled` (Boolean) Whether auto-pause is enabled.

Optional:

- `idle_timeout_minutes` (Number) Idle minutes before auto-pause.

<a id="nestedatt--connection_info"></a>
### Nested Schema for `connection_info`

Read-Only:

- `listener_port` (Number) TCP listener port.
- `private_endpoint` (String) Private endpoint (VPC-internal).
- `public_endpoint` (String) Public endpoint.

<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String) Default: `20m`.
- `delete` (String) Default: `15m`.
- `update` (String) Default: `20m` (extended for resizes and pool add/remove).

## Import

Import uses the composite ID `warehouse_id/cluster_id`:

```shell
terraform import velodb_cluster.example ALBJRXRG/c-m2w789x8kghgpapgaz
```

Or via import block:

```terraform
import {
  to = velodb_cluster.example
  id = "ALBJRXRG/c-m2w789x8kghgpapgaz"
}
```

~> **Note on import:** The API does not return all fields in `GET /clusters/{id}`. After import, these fields are populated from the `billingPools` response (`compute_vcpu`, `cache_gb`, `period`, `period_unit`). `auto_renew` and `auto_pause` are preserved from prior state or default to null — you may need to add them to your configuration explicitly.
