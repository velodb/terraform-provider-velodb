resource "velodb_byoc_credential" "aws" {
  cloud_provider            = "aws"
  name                      = "production-credential"
  region                    = "us-east-1"
  bucket_name               = var.bucket_name
  data_credential_arn       = var.data_credential_arn
  deployment_credential_arn = var.deployment_credential_arn

  # When the IAM policies are created in this configuration, add their
  # aws_iam_role_policy_attachment resources to depends_on.
}

variable "bucket_name" {
  type = string
}

variable "data_credential_arn" {
  description = "AWS IAM instance-profile ARN used by warehouse instances."
  type        = string
}

variable "deployment_credential_arn" {
  description = "AWS IAM role ARN assumed by VeloDB Cloud."
  type        = string
}
