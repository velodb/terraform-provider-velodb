# Standalone example: provide a verified existing warehouse ID.
variable "node_example_warehouse_id" {
  type = string
}

resource "velodb_cluster" "nodes" {
  warehouse_id = var.node_example_warehouse_id
  name         = "node_capacity_example"
  cluster_type = "COMPUTE"
  node_vcpu    = 8
  node_count   = 3
  node_cache   = 200

  # Resize node_vcpu/node_cache in one apply; node_count in a separate apply.
}

# Local NVMe: fixed per-node cache is computed from the selected package.
resource "velodb_cluster" "local_nvme" {
  warehouse_id = var.node_example_warehouse_id
  name         = "local_worker"
  cluster_type = "COMPUTE"
  cache_type   = 1
  ratio        = 4
  node_vcpu    = 4
  node_count   = 2
}
