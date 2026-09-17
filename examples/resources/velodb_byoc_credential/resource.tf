resource "velodb_byoc_credential" "aws" {
  cloud_provider            = "aws"
  name                      = "production-credential"
  region                    = "us-east-1"
  bucket_name               = var.bucket_name
  data_credential_arn       = var.data_credential_arn
  deployment_credential_arn = var.deployment_credential_arn
}

variable "bucket_name" {
  type = string
}

variable "data_credential_arn" {
  type = string
}

variable "deployment_credential_arn" {
  type = string
}
