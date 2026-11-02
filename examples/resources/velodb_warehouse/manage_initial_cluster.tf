# Manage the initial cluster through a separate velodb_cluster resource.
# Create the warehouse first, then verify the physical ID before importing.
# initial_cluster manages capacity after creation; initial_cluster_id is best effort and
# is unavailable after warehouse import. Older cached IDs need verification.

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
  compute_vcpu = 16
  cache_gb     = 400
}

variable "verified_initial_cluster_id" {
  description = "Physical cluster ID verified independently before importing it."
  type        = string
}

# 3. Remove initial_cluster from the existing warehouse configuration to relinquish
# capacity management. Then verify the ID/settings and import the physical cluster.
import {
  to = velodb_cluster.initial
  id = "${velodb_warehouse.main.id}/${var.verified_initial_cluster_id}"
}

resource "velodb_cluster" "initial" {
  warehouse_id = velodb_warehouse.main.id
  name         = "bootstrap"
  cluster_type = "COMPUTE"
  zone         = "us-east-1a"
  compute_vcpu = 4
  cache_gb     = 100
}

# To delete the initial cluster, remove its resource and import blocks.
# Keep at least one other cluster; prepaid clusters require expiration.
