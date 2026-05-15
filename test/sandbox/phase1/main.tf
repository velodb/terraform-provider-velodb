terraform {
  required_providers {
    velodb = {
      source = "velodb/velodb"
    }
  }
}

provider "velodb" {
  host    = "sandbox-api.velodb.io"
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

variable "warehouse_name_override" {
  type    = string
  default = ""
}

variable "warehouse_password" {
  type      = string
  sensitive = true
  default   = "Tf@Migration123"
}

variable "admin_password_version" {
  type    = number
  default = 0
}

variable "core_version_id" {
  type    = number
  default = 0
}

variable "include_cluster" {
  type    = bool
  default = false
}

variable "cluster_desired_state" {
  type    = string
  default = "running"
}

variable "cluster_reboot_trigger" {
  type    = number
  default = 0
}

variable "cluster_vcpu" {
  type    = number
  default = 4
}

variable "cluster_cache_gb_set" {
  type        = bool
  default     = true
  description = "When false, omit cache_gb to let the API auto-scale."
}

variable "cluster_cache_gb" {
  type    = number
  default = 100
}

resource "velodb_warehouse" "t" {
  name                   = coalesce(var.warehouse_name_override, "tfmig-${var.name_suffix}")
  deployment_mode        = "SaaS"
  cloud_provider         = "aws"
  region                 = "us-east-1"
  admin_password         = var.warehouse_password
  admin_password_version = var.admin_password_version > 0 ? var.admin_password_version : null

  core_version_id = var.core_version_id > 0 ? var.core_version_id : null

  initial_cluster {
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

resource "velodb_cluster" "od" {
  count = var.include_cluster ? 1 : 0

  warehouse_id   = velodb_warehouse.t.id
  name           = "od_${var.name_suffix}"
  cluster_type   = "COMPUTE"
  zone           = "us-east-1d"
  desired_state  = var.cluster_desired_state
  reboot_trigger = var.cluster_reboot_trigger
  compute_vcpu   = var.cluster_vcpu
  cache_gb       = var.cluster_cache_gb
}

data "velodb_warehouse_versions" "v" {
  warehouse_id = velodb_warehouse.t.id
}

output "warehouse_id" { value = velodb_warehouse.t.id }
output "core_version" { value = velodb_warehouse.t.core_version }
output "version_default_id" { value = data.velodb_warehouse_versions.v.default_id }
output "version_list" { value = data.velodb_warehouse_versions.v.versions }
output "cluster_id" {
  value = var.include_cluster ? velodb_cluster.od[0].id : null
}
