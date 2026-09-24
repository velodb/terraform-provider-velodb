resource "velodb_byoc_credential" "aws" {
  cloud_provider            = "aws"
  name                      = "production-credential"
  region                    = "us-east-1"
  bucket_name               = var.bucket_name
  data_credential_arn       = var.data_credential_arn
  deployment_credential_arn = var.deployment_credential_arn

  # The warehouse and network both reference this credential, so it is the last
  # VeloDB resource destroyed. Anchor the underlying AWS IAM, storage, and
  # network resources here: on create they are ready before VeloDB validates the
  # credential, and because destroy reverses the graph they are torn down only
  # after the warehouse has finished deleting -- never in parallel with it, which
  # would otherwise strand the backend deletion without S3 or network access.
  # List every AWS resource the running warehouse depends on, for example:
  depends_on = [
    aws_iam_role_policy_attachment.data_access,
    aws_iam_role_policy_attachment.deployment,
    aws_vpc_endpoint.s3,
    aws_vpc_endpoint.velodb,
    aws_route.private_nat,
    aws_vpc_security_group_ingress_rule.warehouse_self,
    aws_vpc_security_group_egress_rule.warehouse_all,
    aws_vpc_security_group_ingress_rule.endpoint_https,
    aws_vpc_security_group_egress_rule.endpoint_all,
  ]
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
