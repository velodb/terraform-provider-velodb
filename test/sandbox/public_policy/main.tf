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

variable "warehouse_id" {
  type = string
}

variable "policy" {
  type    = string
  default = "ALLOWLIST_ONLY"
}

variable "include_rule" {
  type    = bool
  default = true
}

resource "velodb_warehouse_public_access_policy" "current" {
  warehouse_id = var.warehouse_id
  policy       = var.policy

  rules = var.include_rule ? [
    {
      cidr        = "203.0.113.10/32"
      description = "terraform-e2e"
    }
  ] : null
}

output "policy" {
  value = velodb_warehouse_public_access_policy.current.policy
}
