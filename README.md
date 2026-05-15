# Terraform Provider for VeloDB

Terraform provider for the VeloDB Management API.

This tree is aligned to the Management API YAML at
`/Users/zhhanz/Documents/velodb/Skills/managementapi(1).yaml`.

## Provider

```terraform
terraform {
  required_providers {
    velodb = {
      source = "velodb/velodb"
    }
  }
}

provider "velodb" {
  host    = "sandbox-api.velodb.io"
  api_key = var.velodb_api_key
}

variable "velodb_api_key" {
  type      = string
  sensitive = true
}
```

`host` can also be set with `VELODB_HOST`; `api_key` can also be set with
`VELODB_API_KEY`.

## Resources

| Resource | Purpose |
|---|---|
| `velodb_warehouse` | Create/update/delete SaaS or BYOC warehouses |
| `velodb_cluster` | Manage COMPUTE clusters inside a warehouse |
| `velodb_public_access_policy` | Manage public access allowlist policy |
| `velodb_warehouse_private_endpoint` | Register an existing cloud PrivateLink endpoint |
| `velodb_private_link_endpoint_service` | Register/delete reverse endpoint services |

## Data Sources

| Data source | Purpose |
|---|---|
| `velodb_warehouses` | List warehouses using `warehouse_id`, `name`, cloud, region, or deployment mode filters |
| `velodb_clusters` | List clusters using `cluster_id`, `cluster_name`, status, or type filters |
| `velodb_warehouse_connections` | Read public/private endpoints, compute clusters, and observer groups |
| `velodb_warehouse_versions` | List valid warehouse upgrade target versions |

## Warehouse Example

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
```

`deployment_mode` must be `SaaS` or `BYOC`. The current Management API does not
accept `maintenance_window`, `upgrade_policy`, mixed-billing request fields, or
legacy `advanced_settings` in Terraform warehouse/cluster create/update
requests.

## Cluster Example

```terraform
resource "velodb_cluster" "etl" {
  warehouse_id  = velodb_warehouse.main.id
  name          = "etl"
  cluster_type  = "COMPUTE"
  zone          = "us-east-1a"
  compute_vcpu  = 8
  cache_gb      = 200
  desired_state = "running"

  auto_pause {
    enabled              = true
    idle_timeout_minutes = 15
  }
}
```

Pause, resume, and reboot use the explicit Management API endpoints:
`/pause`, `/resume`, and `/reboot`.

## Connections Example

```terraform
data "velodb_warehouse_connections" "main" {
  warehouse_id = velodb_warehouse.main.id
}

output "jdbc_url" {
  value = [for ep in data.velodb_warehouse_connections.main.public_endpoints : ep.url if ep.protocol == "jdbc"][0]
}

output "compute_clusters" {
  value = data.velodb_warehouse_connections.main.compute_clusters
}
```

## Verification

```bash
go test ./...
go install .
terraform -chdir=test/sandbox/phase1 validate -no-color
bash test/sandbox/scripts/phase7.sh
```

For live sandbox phases, set `TF_VAR_api_key` and run the scripts under
`test/sandbox/scripts` in order.
