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

  # EBS: resize node_vcpu/node_cache in one apply; node_count in a separate apply.
}

# Local NVMe: omit node_cache even if it matches the current size.
# For vertical scaling, change only node_vcpu; change node_count separately.
resource "velodb_cluster" "local_nvme" {
  warehouse_id       = var.node_example_warehouse_id
  name               = "local_worker"
  cluster_type       = "COMPUTE"
  cache_storage_type = 1
  ratio              = 4
  node_vcpu          = 4
  node_count         = 2
}
