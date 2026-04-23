# Example: Manage and later delete the initial cluster
#
# The VeloDB API requires an initial_cluster at warehouse creation — a warehouse
# cannot exist without at least one cluster. The initial_cluster block on
# velodb_warehouse is create-only (changes to it after creation are ignored).
#
# To manage the initial cluster post-creation (resize, pause, destroy), import
# it into a separate velodb_cluster resource using the computed
# initial_cluster_id attribute.

variable "admin_password" {
  type      = string
  sensitive = true
}

# 1. Create the warehouse with a small bootstrap cluster
resource "velodb_warehouse" "main" {
  name            = "analytics"
  deployment_mode = "SaaS"
  cloud_provider  = "aws"
  region          = "us-east-1"
  admin_password  = var.admin_password

  initial_cluster {
    name         = "bootstrap"
    zone         = "us-east-1a"
    compute_vcpu = 4
    cache_gb     = 100

    auto_pause {
      enabled              = true
      idle_timeout_minutes = 30
    }
  }
}

# 2. Add a second (real) cluster — required before the initial can be deleted
resource "velodb_cluster" "etl" {
  warehouse_id = velodb_warehouse.main.id
  name         = "etl"
  cluster_type = "COMPUTE"
  zone         = "us-east-1a"

  on_demand {
    compute_vcpu = 16
    cache_gb     = 100
  }
}

# 3. Import the initial cluster so Terraform can manage it
import {
  to = velodb_cluster.initial
  id = "${velodb_warehouse.main.id}/${velodb_warehouse.main.initial_cluster_id}"
}

resource "velodb_cluster" "initial" {
  warehouse_id = velodb_warehouse.main.id
  name         = "bootstrap"
  cluster_type = "COMPUTE"
  zone         = "us-east-1a"

  on_demand {
    compute_vcpu = 4
    cache_gb     = 100
  }
}

# 4. Day-N: destroy the initial cluster by removing both blocks above
#    (resource "velodb_cluster" "initial" and the import block).
#    Then run `terraform apply`.
#
#    Constraints:
#    - The warehouse must still have at least one other cluster
#      (velodb_cluster.etl in this example).
#    - Prepaid (subscription) clusters cannot be deleted until expiration.
