# Standalone example: warehouse initial_cluster fields describe creation only.
variable "node_example_admin_password" {
  type      = string
  sensitive = true
}

resource "velodb_warehouse" "nodes" {
  name            = "node-capacity-example"
  deployment_mode = "SaaS"
  cloud_provider  = "aws"
  region          = "us-east-1"
  admin_password  = var.node_example_admin_password

  initial_cluster {
    zone       = "us-east-1a"
    node_vcpu  = 8
    node_count = 3
    node_cache = 200
  }
}
