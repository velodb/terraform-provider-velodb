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

variable "endpoint_id" {
  type    = string
  default = "vpce-terraform-e2e-missing"
}

resource "velodb_warehouse_private_endpoint" "missing" {
  warehouse_id = var.warehouse_id
  endpoint_id  = var.endpoint_id
  dns_name     = "terraform-e2e.invalid"
  description  = "terraform-e2e negative path"
}
