terraform {
  required_providers {
    velodb = {
      source = "velodb/velodb"
    }
  }
}

provider "velodb" {
  host    = "sandbox.velodb.io"
  api_key = var.api_key
}

variable "api_key" {
  type      = string
  sensitive = true
}

variable "name_suffix" {
  type        = string
  default     = "ci"
  description = "Suffix to keep concurrent runs from colliding."
}

variable "maintenance_start_hour" {
  type    = number
  default = 4
}

variable "maintenance_end_hour" {
  type    = number
  default = 5
}

variable "warehouse_name_override" {
  type    = string
  default = ""
}

resource "velodb_warehouse" "t" {
  name            = coalesce(var.warehouse_name_override, "tfmig-${var.name_suffix}")
  deployment_mode = "SaaS"
  cloud_provider  = "aws"
  region          = "us-east-1"
  admin_password  = "Tf@Migration123"

  upgrade_policy = "automatic"
  maintenance_window = {
    start_hour_utc = var.maintenance_start_hour
    end_hour_utc   = var.maintenance_end_hour
  }

  initial_cluster {
    name         = "default"
    zone         = "us-east-1d"
    compute_vcpu = 4
    cache_gb     = 100
    auto_pause { enabled = false }
  }

  timeouts {
    create = "30m"
    delete = "20m"
  }
}

data "velodb_warehouse_versions" "v" {
  warehouse_id = velodb_warehouse.t.id
}

output "warehouse_id"       { value = velodb_warehouse.t.id }
output "core_version"       { value = velodb_warehouse.t.core_version }
output "maintenance_window" { value = velodb_warehouse.t.maintenance_window }
output "upgrade_policy"     { value = velodb_warehouse.t.upgrade_policy }
output "version_default_id" { value = data.velodb_warehouse_versions.v.default_id }
output "version_list"       { value = data.velodb_warehouse_versions.v.versions }
