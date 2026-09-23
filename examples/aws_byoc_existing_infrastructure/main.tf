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
  source = "../../modules/aws_byoc_existing"

  region                    = var.region
  bucket_name               = var.bucket_name
  name_prefix               = var.name_prefix
  admin_password            = var.admin_password
  vpc_id                    = var.vpc_id
  subnet_ids_by_zone        = var.subnet_ids_by_zone
  security_group_id         = var.security_group_id
  endpoint_id               = var.endpoint_id
  data_credential_arn       = var.data_credential_arn
  deployment_credential_arn = var.deployment_credential_arn
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

output "aws_resources_managed" {
  value = module.velodb_byoc.aws_resources_managed
}
