terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    velodb = {
      source  = "velodb/velodb"
      version = ">= 1.1.8"
    }
  }
}

provider "aws" {
  region = var.region
}

provider "velodb" {}

module "velodb_byoc" {
  source = "../../modules/aws_byoc"

  region         = var.region
  bucket_name    = var.bucket_name
  name_prefix    = var.name_prefix
  admin_password = var.admin_password
  vpc_cidr       = var.vpc_cidr
  zones          = var.zones
  tags           = var.tags
}

output "warehouse_id" {
  value = module.velodb_byoc.warehouse_id
}

output "warehouse_status" {
  value = module.velodb_byoc.warehouse_status
}

output "initial_cluster_id" {
  value = module.velodb_byoc.initial_cluster_id
}

output "vpc_id" {
  value = module.velodb_byoc.vpc_id
}
