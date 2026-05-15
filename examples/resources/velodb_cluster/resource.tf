# Compute cluster — always running
resource "velodb_cluster" "etl" {
  warehouse_id   = velodb_warehouse.saas.id
  name           = "compute_etl"
  cluster_type   = "COMPUTE"
  zone           = "cn-beijing-k"
  compute_vcpu   = 4
  cache_gb       = 100
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

# Dev cluster — paused by default for cost savings
resource "velodb_cluster" "dev" {
  warehouse_id   = velodb_warehouse.saas.id
  name           = "compute_dev"
  cluster_type   = "COMPUTE"
  zone           = "cn-beijing-k"
  compute_vcpu   = 4
  cache_gb       = 100
  desired_state  = "paused"

  auto_pause {
    enabled              = true
    idle_timeout_minutes = 5
  }
}

# Output connection info
output "etl_endpoint" {
  value = velodb_cluster.etl.connection_info[0].public_endpoint
}
