terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 6.24.0"
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

  region                 = var.region
  bucket_name            = var.bucket_name
  name_prefix            = var.name_prefix
  admin_password         = var.admin_password
  vpc_cidr               = var.vpc_cidr
  zones                  = var.zones
  subnet_cidrs           = var.subnet_cidrs
  warehouse_client_cidrs = var.warehouse_client_cidrs
  additional_clusters    = var.additional_clusters
  tags                   = var.tags
  initial_core_version   = var.initial_core_version
  public_access_policy   = var.public_access_policy

  create_tde_encryption_key = var.create_tde_encryption_key
  tde_kms_key_arn           = var.tde_kms_key_arn
  create_ebs_encryption_key = var.create_ebs_encryption_key
  ebs_kms_key_arn           = var.ebs_kms_key_arn
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

output "additional_cluster_ids" {
  value = module.velodb_byoc.additional_cluster_ids
}

output "vpc_id" {
  value = module.velodb_byoc.vpc_id
}

output "tde_encryption_key_id" {
  value = module.velodb_byoc.tde_encryption_key_id
}

output "ebs_encryption_key_id" {
  value = module.velodb_byoc.ebs_encryption_key_id
}
