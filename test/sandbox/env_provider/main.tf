terraform {
  required_providers {
    velodb = {
      source = "velodb/velodb"
    }
  }
}

provider "velodb" {}

variable "warehouse_name" {
  type = string
}

data "velodb_warehouses" "env" {
  name            = var.warehouse_name
  cloud_provider  = "aws"
  region          = "us-east-1"
  deployment_mode = "SaaS"
}

output "total" {
  value = data.velodb_warehouses.env.total
}
