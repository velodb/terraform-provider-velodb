# Terraform Provider for VeloDB

The VeloDB Terraform provider manages VeloDB Cloud warehouses, compute clusters,
network access, PrivateLink registrations, and connection metadata through the
VeloDB Cloud Management API.

## Get Started

### 1. Get a VeloDB Cloud API key

Go to the VeloDB Cloud console, then open **Organization -> API Keys**.

Console URL: <https://www.velodb.cloud/organization/api-keys>

Click **Create API Key**, choose the role and expiration, then copy the key when
it is generated. VeloDB shows the raw key only once. Keys start with `sk-`.

Store the key in an environment variable, not in Terraform files, chat, shell
history, or source control:

```bash
export VELODB_API_KEY='sk-...'
```

### 2. Use the VeloDB Cloud API host

The default VeloDB Cloud Management API host is:

```text
api.velodb.cloud
```

`host` is a bare hostname. Do not include `https://`; the provider adds HTTPS
for non-local hosts.

```bash
export VELODB_HOST='api.velodb.cloud'
```

### 3. Configure Terraform

```terraform
terraform {
  required_providers {
    velodb = {
      source  = "velodb/velodb"
      version = "~> 1.1"
    }
  }
}

provider "velodb" {
  host    = var.velodb_host
  api_key = var.velodb_api_key
}

variable "velodb_host" {
  type        = string
  description = "VeloDB Cloud Management API host, without https://."
  default     = "api.velodb.cloud"
}

variable "velodb_api_key" {
  type        = string
  description = "VeloDB Cloud API key."
  sensitive   = true
}
```

Environment variables are also supported:

```bash
export VELODB_HOST='api.velodb.cloud'
export VELODB_API_KEY='sk-...'
terraform plan
```

## Example Usage

Create a SaaS warehouse with its initial compute cluster:

```terraform
resource "velodb_warehouse" "analytics" {
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
```

Add a compute cluster:

```terraform
resource "velodb_cluster" "etl" {
  warehouse_id  = velodb_warehouse.analytics.id
  name          = "etl"
  cluster_type  = "COMPUTE"
  zone          = "us-east-1a"
  compute_vcpu  = 8
  cache_gb      = 200
  desired_state = "running"
}
```

Read connection information for applications and automation:

```terraform
data "velodb_warehouse_connections" "analytics" {
  warehouse_id = velodb_warehouse.analytics.id
}

output "jdbc_urls" {
  value = [
    for ep in data.velodb_warehouse_connections.analytics.public_endpoints :
    ep.url if ep.protocol == "jdbc"
  ]
}
```

Import an existing BYOC warehouse:

```terraform
import {
  to = velodb_warehouse.byoc
  id = "AWVA7PYB"
}

resource "velodb_warehouse" "byoc" {
  name            = "test_cli"
  deployment_mode = "BYOC"
  cloud_provider  = "aws"
  region          = "us-east-1"
}
```

## Resources

| Resource | Purpose |
|---|---|
| `velodb_warehouse` | Create, update, and delete SaaS warehouses; import and read existing BYOC warehouses. |
| `velodb_cluster` | Manage COMPUTE clusters inside a warehouse, including resize, pause, resume, and reboot. |
| `velodb_warehouse_public_access_policy` | Manage public endpoint access policy and CIDR allowlists. |
| `velodb_warehouse_private_endpoint` | Register and describe inbound PrivateLink endpoints for warehouse access. |
| `velodb_private_link_endpoint_service` | Register external endpoint services that VeloDB Cloud can access through PrivateLink. |

## Data Sources

| Data source | Purpose |
|---|---|
| `velodb_warehouses` | List warehouses by ID, name, cloud provider, region, or deployment mode. |
| `velodb_clusters` | List clusters in a warehouse by ID, name, status, type, or billing model. |
| `velodb_warehouse_connections` | Read public/private endpoints, compute clusters, observer groups, and PrivateLink service names. |
| `velodb_warehouse_versions` | List valid warehouse upgrade target version IDs. |
| `velodb_private_link_endpoint_services` | List outbound PrivateLink endpoint services and connected endpoints. |

## Known Limitations

- BYOC warehouses can be imported and read, but this provider does not create
  new BYOC warehouses. Create BYOC warehouses in VeloDB Cloud, then import the
  warehouse ID into Terraform.
- `velodb_cluster` manages `COMPUTE` clusters. `SQL` and `OBSERVER` cluster
  types are blocked at plan time.
- CPU and cache resize are applied one dimension at a time. When increasing
  `compute_vcpu`, set `cache_gb` to the API-implied minimum for the new CPU
  size, then apply any additional cache-only change in a later run.
- The current Management API does not accept `maintenance_window`,
  `upgrade_policy`, mixed-billing request fields, or legacy
  `advanced_settings` in Terraform warehouse or cluster create/update requests.
- `admin_password` is write-only in the API and is stored in Terraform state as
  a sensitive value so Terraform can detect password rotation.

## Development

```bash
go test ./...
```

Live tests require `VELODB_API_KEY`. Set `VELODB_HOST` only when overriding the
default `api.velodb.cloud` host.
