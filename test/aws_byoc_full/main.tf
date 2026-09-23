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

variable "region" {
  type    = string
  default = "us-east-1"
}

variable "bucket_name" {
  description = "Globally unique disposable S3 bucket name."
  type        = string
}

variable "name_prefix" {
  description = "Prefix for disposable AWS and VeloDB resources."
  type        = string
  default     = "tf-byoc-live"
}

variable "admin_password" {
  description = "Warehouse administrator password. Set with TF_VAR_admin_password."
  type        = string
  sensitive   = true
}

variable "create_second_cluster" {
  type    = bool
  default = false
}

variable "vpc_cidr" {
  type    = string
  default = "10.57.0.0/16"
}

variable "zones" {
  type    = list(string)
  default = ["us-east-1a", "us-east-1b", "us-east-1d"]
}

module "byoc" {
  source = "../../modules/aws_byoc"

  region                = var.region
  bucket_name           = var.bucket_name
  name_prefix           = var.name_prefix
  admin_password        = var.admin_password
  vpc_cidr              = var.vpc_cidr
  zones                 = var.zones
  create_second_cluster = var.create_second_cluster
  bucket_force_destroy  = true
  tags                  = { purpose = "velodb-provider-live-test" }
}

output "test_status" {
  value = {
    aws_account_id     = module.byoc.aws_account_id
    zones              = module.byoc.zones
    bucket             = module.byoc.bucket_name
    vpc_id             = module.byoc.vpc_id
    private_subnet_ids = module.byoc.private_subnet_ids
    endpoint_id        = module.byoc.endpoint_id
    credential_id      = module.byoc.credential_id
    network_config_id  = module.byoc.network_config_id
    warehouse_id       = module.byoc.warehouse_id
    warehouse_status   = module.byoc.warehouse_status
    initial_cluster_id = module.byoc.initial_cluster_id
    second_cluster_id  = module.byoc.second_cluster_id
  }
}
