# VeloDB Terraform Provider Documentation

The VeloDB provider manages warehouses, clusters, and related infrastructure on [VeloDB Cloud](https://www.selectdb.com/) (SelectDB Cloud) using the Formation OpenAPI.

**Resources:**

| Resource | Description |
|---|---|
| [velodb_warehouse](#velodb_warehouse-resource) | Manages a warehouse (SAAS or BYOC) |
| [velodb_cluster](#velodb_cluster-resource) | Manages a cluster within a warehouse |

**Data Sources:**

| Data Source | Description |
|---|---|
| [velodb_warehouses](#velodb_warehouses-data-source) | Lists warehouses with optional filters |
| [velodb_clusters](#velodb_clusters-data-source) | Lists clusters within a warehouse |
| [velodb_warehouse_connections](#velodb_warehouse_connections-data-source) | Gets JDBC/HTTP/stream-load endpoints for a warehouse |

---

## Provider Configuration

```terraform
terraform {
  required_providers {
    velodb = {
      source  = "velodb/velodb"
      version = "~> 0.1"
    }
  }
}

provider "velodb" {
  host    = "api.selectdbcloud.com"   # or VELODB_HOST env var
  api_key = var.velodb_api_key         # or VELODB_API_KEY env var
}

variable "velodb_api_key" {
  type      = string
  sensitive = true
}
```

### Provider Schema

| Attribute | Type | Required | Sensitive | Description |
|---|---|---|---|---|
| `host` | String | No | No | Formation API host. Falls back to `VELODB_HOST` env var. |
| `api_key` | String | No | Yes | API key for authentication. Falls back to `VELODB_API_KEY` env var. |

---

## Complete Example

This example creates a SAAS warehouse with an initial SQL cluster, adds a COMPUTE cluster for ETL, a paused dev cluster for cost savings, and outputs the connection endpoints.

```terraform
terraform {
  required_providers {
    velodb = {
      source  = "velodb/velodb"
      version = "~> 0.1"
    }
  }
}

provider "velodb" {
  host    = var.velodb_host
  api_key = var.velodb_api_key
}

# ─── Variables ───────────────────────────────────────────────

variable "velodb_host" {
  type    = string
  default = "api.selectdbcloud.com"
}

variable "velodb_api_key" {
  type      = string
  sensitive = true
}

variable "admin_password" {
  type      = string
  sensitive = true
}

# ─── Warehouse ───────────────────────────────────────────────

resource "velodb_warehouse" "main" {
  name            = "analytics"
  deployment_mode = "SAAS"
  cloud_provider  = "aliyun"
  region          = "cn-beijing"

  admin_password         = var.admin_password
  admin_password_version = 1

  maintainability_start_time = "02:00"
  maintainability_end_time   = "06:00"

  advanced_settings = jsonencode({
    enableTde = 0
  })

  initial_cluster {
    name         = "sql-primary"
    zone         = "cn-beijing-k"
    compute_vcpu = 16
    cache_gb     = 800

    auto_pause {
      enabled = false
    }
  }

  tags = {
    environment = "production"
    team        = "data-platform"
  }

  timeouts {
    create = "30m"
    delete = "20m"
  }
}

# ─── ETL Cluster (always running, auto-pauses after idle) ────

resource "velodb_cluster" "etl" {
  warehouse_id   = velodb_warehouse.main.id
  name           = "compute-etl"
  cluster_type   = "COMPUTE"
  zone           = "cn-beijing-k"
  compute_vcpu   = 32
  cache_gb       = 1600
  billing_method = "on_demand"
  desired_state  = "running"

  auto_pause {
    enabled              = true
    idle_timeout_minutes = 15
  }

  timeouts {
    create = "20m"
    update = "20m"
  }
}

# ─── Dev Cluster (paused by default for cost savings) ────────

resource "velodb_cluster" "dev" {
  warehouse_id   = velodb_warehouse.main.id
  name           = "compute-dev"
  cluster_type   = "COMPUTE"
  zone           = "cn-beijing-k"
  compute_vcpu   = 4
  cache_gb       = 100
  billing_method = "on_demand"
  desired_state  = "paused"

  auto_pause {
    enabled              = true
    idle_timeout_minutes = 5
  }
}

# ─── Data Sources ────────────────────────────────────────────

data "velodb_warehouse_connections" "main" {
  warehouse_id = velodb_warehouse.main.id
}

data "velodb_warehouses" "all_prod" {
  cloud_provider  = "aliyun"
  region          = "cn-beijing"
  deployment_mode = "SAAS"
}

data "velodb_clusters" "running" {
  warehouse_id = velodb_warehouse.main.id
  status       = "Running"
  cluster_type = "COMPUTE"
}

# ─── Outputs ─────────────────────────────────────────────────

output "warehouse_id" {
  value = velodb_warehouse.main.id
}

output "warehouse_status" {
  value = velodb_warehouse.main.status
}

output "etl_endpoint" {
  value = velodb_cluster.etl.connection_info[0].public_endpoint
}

output "jdbc_url" {
  value = "jdbc:mysql://${data.velodb_warehouse_connections.main.clusters[0].public_endpoint}:${data.velodb_warehouse_connections.main.clusters[0].jdbc_port}"
}

output "http_url" {
  value = "http://${data.velodb_warehouse_connections.main.clusters[0].public_endpoint}:${data.velodb_warehouse_connections.main.clusters[0].http_port}"
}

output "total_warehouses" {
  value = data.velodb_warehouses.all_prod.total
}

output "running_compute_clusters" {
  value = [for cl in data.velodb_clusters.running.clusters : cl.name]
}
```

---

## velodb_warehouse (Resource)

Manages a VeloDB Cloud warehouse. A warehouse is the top-level compute and storage unit that contains one or more clusters.

### Example: SaaS Warehouse

```terraform
resource "velodb_warehouse" "analytics" {
  name            = "analytics-saas"
  deployment_mode = "SAAS"
  cloud_provider  = "aliyun"
  region          = "cn-beijing"

  admin_password         = var.admin_password
  admin_password_version = 1

  advanced_settings = jsonencode({ enableTde = 0 })

  initial_cluster {
    name         = "default"
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
  }
}
```

### Example: BYOC Warehouse with Template Mode

```terraform
resource "velodb_warehouse" "production" {
  name            = "production-byoc"
  deployment_mode = "BYOC"
  cloud_provider  = "aliyun"
  region          = "cn-beijing"
  setup_mode     = "guided"
  vpc_mode        = "existing"
  vpc_id          = "vpc-2ze1234567890abcdef"

  admin_password         = var.admin_password
  admin_password_version = 1

  core_version = "3.0.3"

  maintainability_start_time = "02:00"
  maintainability_end_time   = "06:00"

  initial_cluster {
    name           = "default-compute"
    zone           = "cn-beijing-k"
    compute_vcpu   = 8
    cache_gb       = 400
    billing_method = "monthly"
    auto_pause {
      enabled              = true
      idle_timeout_minutes = 30
    }
  }

  tags = {
    environment = "production"
    team        = "data-platform"
  }

  timeouts {
    create = "45m"
  }
}

output "byoc_shell_command" {
  value     = velodb_warehouse.production.byoc_setup[0].shell_command
  sensitive = true
}
```

### Example: BYOC Warehouse with Wizard Mode (AWS)

```terraform
resource "velodb_warehouse" "aws_byoc" {
  name            = "aws-byoc-wizard"
  deployment_mode = "BYOC"
  cloud_provider  = "aws"
  region          = "us-east-1"
  setup_mode     = "advanced"

  credential_id             = 12345
  network_config_id         = 67890
  bucket_name               = "my-velodb-bucket"
  data_credential_arn       = "arn:aws:iam::123456789012:role/velodb-data"
  deployment_credential_arn = "arn:aws:iam::123456789012:role/velodb-deploy"
  subnet_id                 = "subnet-0abc123def456"
  security_group_id         = "sg-0abc123def456"

  admin_password         = var.admin_password
  admin_password_version = 1

  initial_cluster {
    name         = "sql-primary"
    zone         = "us-east-1a"
    compute_vcpu = 16
    cache_gb     = 800
  }

  timeouts {
    create = "45m"
  }
}
```

### Example: Password Rotation

Change `admin_password` and increment `admin_password_version`:

```terraform
resource "velodb_warehouse" "example" {
  # ...existing config...
  admin_password         = var.new_password  # changed
  admin_password_version = 2                  # bumped from 1
}
```

### Example: Version Upgrade

Change `core_version` — the provider calls the upgrade API and waits:

```terraform
resource "velodb_warehouse" "example" {
  # ...existing config...
  core_version = "3.1.0"  # was "3.0.3"
}
```

### Example: Manage / delete the initial cluster

The API requires `initial_cluster` at creation time, but you may want to delete or resize it later. Import it as a `velodb_cluster` resource using the computed `initial_cluster_id`:

```terraform
resource "velodb_warehouse" "main" {
  name            = "analytics"
  deployment_mode = "SaaS"
  cloud_provider  = "aws"
  region          = "us-east-1"
  admin_password  = var.admin_password

  initial_cluster {
    name         = "bootstrap"
    compute_vcpu = 4
    cache_gb     = 100
  }
}

# Add a second cluster first (API won't let you delete the last cluster)
resource "velodb_cluster" "etl" {
  warehouse_id = velodb_warehouse.main.id
  name         = "etl"
  cluster_type = "COMPUTE"
  on_demand { compute_vcpu = 16, cache_gb = 100 }
}

# Import the initial cluster for management
import {
  to = velodb_cluster.initial
  id = "${velodb_warehouse.main.id}/${velodb_warehouse.main.initial_cluster_id}"
}

resource "velodb_cluster" "initial" {
  warehouse_id = velodb_warehouse.main.id
  name         = "bootstrap"
  cluster_type = "COMPUTE"
  on_demand { compute_vcpu = 4, cache_gb = 100 }
}

# To destroy the initial cluster: remove the resource + import blocks, then apply.
# Constraints:
#   - The warehouse must still have at least one other cluster.
#   - Prepaid (subscription) clusters can't be deleted until expiration.
```

### Schema

#### Required

- `cloud_provider` (String) Cloud provider (e.g., `aws`, `aliyun`). Changing this forces a new resource.
- `deployment_mode` (String) Deployment mode: `BYOC` or `SAAS`. Changing this forces a new resource.
- `name` (String) Warehouse display name.
- `region` (String) Cloud region (e.g., `us-east-1`, `cn-beijing`). Changing this forces a new resource.

#### Optional

- `admin_password` (String, Sensitive) Administrator password. Set on creation, used for password rotation.
- `admin_password_version` (Number) Increment to trigger a password change. Must be used with `admin_password`.
- `advanced_settings` (String) Advanced settings as a JSON string. Use `jsonencode()`.
- `bucket_name` (String) Object storage bucket name for Wizard mode. Forces new resource.
- `core_version` (String) Core version. Changing triggers an upgrade workflow. Computed if not set.
- `setup_mode` (String) BYOC creation mode: `Template` or `Wizard`. Forces new resource.
- `credential_id` (Number) Credential identifier for Wizard mode. Forces new resource.
- `data_credential_arn` (String) Data plane credential ARN. Forces new resource.
- `deployment_credential_arn` (String) Deployment credential ARN. Forces new resource.
- `endpoint_id` (String) Private endpoint identifier. Forces new resource.
- `maintainability_end_time` (String) Maintenance window end time (e.g., `06:00`).
- `maintainability_start_time` (String) Maintenance window start time (e.g., `02:00`).
- `network_config_id` (Number) Network configuration identifier for Wizard mode. Forces new resource.
- `security_group_id` (String) Security group identifier. Forces new resource.
- `subnet_id` (String) Subnet identifier. Forces new resource.
- `tags` (Map of String) Warehouse tags. Set at creation time.
- `vpc_id` (String) VPC identifier for Template mode. Forces new resource.
- `vpc_mode` (String) VPC hint for Template mode: `existing` or `new`. Forces new resource.

#### Read-Only

- `byoc_setup` (List of Object) BYOC setup guidance for BYOC warehouses. Each item contains: `token`, `shell_command`, `shell_command_for_new_vpc`, `url`, `doc_url`, `url_for_new_vpc`, `doc_url_for_new_vpc`.
- `created_at` (String) Creation time in RFC 3339 format.
- `expire_time` (String) Expiration time when available.
- `id` (String) Warehouse identifier (e.g., `ALBJ07YE`).
- `initial_cluster_id` (String) ID of the initial cluster. Use with an `import {}` block to manage or delete the initial cluster as a `velodb_cluster` resource (see example above).
- `pay_type` (String) Billing type: `PostPaid` or `PrePaid`.
- `status` (String) Current status: `Creating`, `Running`, `Resizing`, `Adjusting`, `Upgrading`, `Suspending`, `Resuming`, `Stopping`, `Starting`, `Restarting`, `Deleting`, `Suspended`, `Stopped`, `Deleted`, `CreateFailed`.
- `zone` (String) Primary availability zone.

#### Nested: `initial_cluster`

Create-only block for the cluster provisioned with the warehouse.

| Attribute | Type | Required | Description |
|---|---|---|---|
| `name` | String | Yes | Cluster name |
| `compute_vcpu` | Number | Yes | Compute vCPUs |
| `cache_gb` | Number | Yes | Cache capacity in GB |
| `zone` | String | No | Availability zone |
| `billing_method` | String | No | `monthly` or `on_demand` |
| `period` | Number | No | Prepaid subscription length |
| `period_unit` | String | No | `Month`, `Year`, or `Week` |

#### Nested: `initial_cluster.auto_pause`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `enabled` | Boolean | Yes | Whether auto-pause is enabled |
| `idle_timeout_minutes` | Number | No | Idle minutes before auto-pause |

#### Nested: `byoc_setup` (Read-Only)

| Attribute | Type | Description |
|---|---|---|
| `token` | String | Short-lived BYOC setup token |
| `shell_command` | String | Shell command for provider-side setup |
| `shell_command_for_new_vpc` | String | Shell command for new-VPC setup path |
| `url` | String | Guided setup URL |
| `doc_url` | String | Documentation URL |
| `url_for_new_vpc` | String | Setup URL for new-VPC path |
| `doc_url_for_new_vpc` | String | Doc URL for new-VPC path |

#### Timeouts

| Operation | Default |
|---|---|
| `create` | 45 minutes |
| `update` | 15 minutes |
| `delete` | 20 minutes |

### Import

```shell
terraform import velodb_warehouse.example ALBJ07YE
```

```terraform
import {
  to = velodb_warehouse.example
  id = "ALBJ07YE"
}
```

> **Note:** `admin_password`, `admin_password_version`, `initial_cluster`, and `advanced_settings` are not populated on import.

---

## velodb_cluster (Resource)

Manages a cluster within a VeloDB Cloud warehouse. Clusters are the compute units that run queries.

### Example: Basic Compute Cluster

```terraform
resource "velodb_cluster" "etl" {
  warehouse_id   = velodb_warehouse.main.id
  name           = "compute-etl"
  cluster_type   = "COMPUTE"
  zone           = "cn-beijing-k"
  compute_vcpu   = 4
  cache_gb       = 100
  billing_method = "on_demand"
  desired_state  = "running"

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

### Example: Paused Dev Cluster

```terraform
resource "velodb_cluster" "dev" {
  warehouse_id   = velodb_warehouse.main.id
  name           = "compute-dev"
  cluster_type   = "COMPUTE"
  zone           = "cn-beijing-k"
  compute_vcpu   = 4
  cache_gb       = 100
  billing_method = "on_demand"
  desired_state  = "paused"

  auto_pause {
    enabled              = true
    idle_timeout_minutes = 5
  }
}
```

### Example: Prepaid Monthly Cluster

```terraform
resource "velodb_cluster" "prepaid" {
  warehouse_id       = velodb_warehouse.main.id
  name               = "sql-primary"
  cluster_type       = "SQL"
  zone               = "cn-beijing-k"
  compute_vcpu       = 16
  cache_gb           = 800
  billing_method     = "monthly"
  period             = 1
  period_unit        = "Month"
  auto_renew_enabled = 1
  desired_state      = "running"

  auto_pause {
    enabled = false
  }
}
```

### Day-2 Operations

**Resize** — change `compute_vcpu` and/or `cache_gb`:

```terraform
resource "velodb_cluster" "etl" {
  # ...
  compute_vcpu = 8    # was 4
  cache_gb     = 500  # was 100
}
```

**Pause** — change `desired_state`:

```terraform
resource "velodb_cluster" "etl" {
  # ...
  desired_state = "paused"   # triggers POST /actions {"action":"pause"}
}
```

**Resume** — change back:

```terraform
resource "velodb_cluster" "etl" {
  # ...
  desired_state = "running"  # triggers POST /actions {"action":"resume"}
}
```

### desired_state Behavior

| Current Status | `desired_state = "running"` | `desired_state = "paused"` |
|---|---|---|
| Running | no-op | calls `pause` → Suspended |
| Suspended | calls `resume` → Running | no-op |
| Stopped | calls `resume` → Running | no-op |

### Schema

#### Required

- `cache_gb` (Number) Cache capacity in GB. Changing triggers resize.
- `cluster_type` (String) `SQL`, `COMPUTE`, or `OBSERVER`. Forces new resource.
- `compute_vcpu` (Number) Compute vCPUs. Changing triggers resize.
- `name` (String) Cluster display name.
- `warehouse_id` (String) Parent warehouse identifier. Forces new resource.

#### Optional

- `auto_renew_enabled` (Number) Auto-renew flag for prepaid billing.
- `billing_method` (String) `on_demand` or `monthly`.
- `desired_state` (String) `running` or `paused`. Changes trigger cluster actions.
- `period` (Number) Prepaid subscription length.
- `period_unit` (String) `Month`, `Year`, or `Week`.
- `zone` (String) Availability zone. Forces new resource.

#### Read-Only

- `cloud_provider` (String) Inherited from parent warehouse.
- `connection_info` (List of Object) Each item: `public_endpoint` (String), `private_endpoint` (String), `listener_port` (Number).
- `created_at` (String) Creation time in RFC 3339 format.
- `disk_sum_size` (Number) Current disk capacity in GB.
- `expire_time` (String) Expiration time when available.
- `id` (String) Cluster identifier (e.g., `c-1997tallv8chbkdhej`).
- `pay_type` (String) `PostPaid` or `PrePaid`.
- `region` (String) Inherited from parent warehouse.
- `started_at` (String) Start time in RFC 3339 format.
- `status` (String) Current observed status: `Creating`, `Running`, `Resizing`, `Adjusting`, `Upgrading`, `Suspending`, `Resuming`, `Stopping`, `Starting`, `Restarting`, `Deleting`, `Suspended`, `Stopped`, `Deleted`, `CreateFailed`.

#### Nested: `auto_pause`

| Attribute | Type | Required | Description |
|---|---|---|---|
| `enabled` | Boolean | Yes | Whether auto-pause is enabled |
| `idle_timeout_minutes` | Number | No | Idle minutes before auto-pause |

#### Nested: `connection_info` (Read-Only)

| Attribute | Type | Description |
|---|---|---|
| `public_endpoint` | String | Public endpoint address |
| `private_endpoint` | String | Private endpoint for VPC-internal access |
| `listener_port` | Number | TCP listener port |

#### Timeouts

| Operation | Default |
|---|---|
| `create` | 20 minutes |
| `update` | 20 minutes |
| `delete` | 15 minutes |

### Import

```shell
# Format: warehouse_id/cluster_id
terraform import velodb_cluster.example ALBJRXRG/c-m2w789x8kghgpapgaz
```

```terraform
import {
  to = velodb_cluster.example
  id = "ALBJRXRG/c-m2w789x8kghgpapgaz"
}
```

> **Note:** `compute_vcpu`, `cache_gb`, `billing_method`, `desired_state`, and `auto_pause` are not populated on import.

---

## velodb_warehouses (Data Source)

Lists warehouses visible to the current organization with optional filters.

### Example

```terraform
data "velodb_warehouses" "beijing_saas" {
  cloud_provider  = "aliyun"
  region          = "cn-beijing"
  deployment_mode = "SAAS"
}

output "warehouse_names" {
  value = [for wh in data.velodb_warehouses.beijing_saas.warehouses : wh.name]
}

output "warehouse_count" {
  value = data.velodb_warehouses.beijing_saas.total
}
```

### Schema

#### Optional

- `cloud_provider` (String) Cloud provider filter.
- `deployment_mode` (String) `BYOC` or `SAAS`.
- `keyword` (String) Fuzzy match on warehouse name or ID.
- `region` (String) Cloud region filter.

#### Read-Only

- `total` (Number) Total matching warehouses.
- `warehouses` (List of Object) Each item: `warehouse_id`, `name`, `status`, `cloud_provider`, `region`, `zone`, `deployment_mode`, `core_version`, `pay_type`, `created_at`, `expire_time`.

---

## velodb_clusters (Data Source)

Lists clusters within a warehouse with optional filters.

### Example

```terraform
data "velodb_clusters" "running_compute" {
  warehouse_id = velodb_warehouse.main.id
  status       = "Running"
  cluster_type = "COMPUTE"
}

output "cluster_names" {
  value = [for cl in data.velodb_clusters.running_compute.clusters : cl.name]
}
```

### Schema

#### Required

- `warehouse_id` (String) Parent warehouse identifier.

#### Optional

- `cluster_type` (String) `SQL`, `COMPUTE`, or `OBSERVER`.
- `keyword` (String) Fuzzy match on cluster name or ID.
- `pay_type` (String) `PostPaid` or `PrePaid`.
- `status` (String) Status filter (e.g., `Running`, `Suspended`).

#### Read-Only

- `clusters` (List of Object) Each item: `cluster_id`, `warehouse_id`, `name`, `status`, `cluster_type`, `cloud_provider`, `region`, `zone`, `disk_sum_size`, `pay_type`, `created_at`, `started_at`, `expire_time`.
- `total` (Number) Total matching clusters.

---

## velodb_warehouse_connections (Data Source)

Gets connection endpoints (JDBC, HTTP, stream load) for all clusters in a warehouse.

### Example

```terraform
data "velodb_warehouse_connections" "prod" {
  warehouse_id = velodb_warehouse.production.id
}

output "jdbc_url" {
  value = "jdbc:mysql://${data.velodb_warehouse_connections.prod.clusters[0].public_endpoint}:${data.velodb_warehouse_connections.prod.clusters[0].jdbc_port}"
}

output "http_url" {
  value = "http://${data.velodb_warehouse_connections.prod.clusters[0].public_endpoint}:${data.velodb_warehouse_connections.prod.clusters[0].http_port}"
}

output "stream_load_url" {
  value = "http://${data.velodb_warehouse_connections.prod.clusters[0].public_endpoint}:${data.velodb_warehouse_connections.prod.clusters[0].stream_load_port}"
}

output "private_jdbc_url" {
  value = "jdbc:mysql://${data.velodb_warehouse_connections.prod.clusters[0].private_endpoint}:${data.velodb_warehouse_connections.prod.clusters[0].jdbc_port}"
}

# Iterate over all clusters
output "all_endpoints" {
  value = {
    for cl in data.velodb_warehouse_connections.prod.clusters :
    cl.cluster_id => {
      type             = cl.type
      public_endpoint  = cl.public_endpoint
      private_endpoint = cl.private_endpoint
      jdbc_port        = cl.jdbc_port
      http_port        = cl.http_port
      stream_load_port = cl.stream_load_port
    }
  }
}
```

### Schema

#### Required

- `warehouse_id` (String) Warehouse identifier.

#### Read-Only

- `clusters` (List of Object) Connection info per cluster. Each item:

| Attribute | Type | Description |
|---|---|---|
| `cluster_id` | String | Cluster identifier |
| `type` | String | Cluster type (`SQL`, `COMPUTE`, `OBSERVER`) |
| `jdbc_port` | Number | JDBC port for SQL access |
| `http_port` | Number | HTTP API port |
| `stream_load_port` | Number | Stream load port for bulk ingestion |
| `public_endpoint` | String | Public endpoint address |
| `private_endpoint` | String | Private endpoint for VPC-internal access |
| `listener_port` | Number | TCP listener port |
| `endpoint_service_id` | String | Endpoint service identifier for private link |
