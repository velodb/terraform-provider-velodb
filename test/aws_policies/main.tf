terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    velodb = {
      source  = "velodb/velodb"
      version = ">= 1.1.7"
    }
  }
}

provider "aws" {
  region = var.region
}

provider "velodb" {}

variable "region" {
  description = "Commercial AWS region used to read VeloDB prerequisites."
  type        = string
  default     = "us-east-1"
}

variable "bucket_name" {
  description = "S3 bucket name embedded in the generated policies; this test does not create the bucket."
  type        = string
}

variable "name_prefix" {
  description = "Prefix for disposable IAM resources."
  type        = string
  default     = "velodb-policy-live-test"

  validation {
    condition     = length(var.name_prefix) <= 40 && can(regex("^[A-Za-z0-9+=,.@_-]+$", var.name_prefix))
    error_message = "name_prefix must be at most 40 IAM-safe characters."
  }
}

variable "tde_kms_arn" {
  description = "Optional commercial AWS KMS key ARN used to exercise the TDE statement."
  type        = string
  default     = null
}

data "aws_caller_identity" "current" {}

data "velodb_byoc_prerequisites" "aws" {
  cloud_provider = "aws"
  region         = var.region
}

locals {
  account_id              = data.aws_caller_identity.current.account_id
  data_access_role_name   = "${var.name_prefix}-data-access"
  deployment_role_name    = "${var.name_prefix}-deployment"
  data_access_role_arn    = "arn:aws:iam::${local.account_id}:role/${local.data_access_role_name}"
  data_access_profile_arn = "arn:aws:iam::${local.account_id}:instance-profile/${local.data_access_role_name}"
}

data "velodb_aws_data_access_assume_role_policy" "data_access" {
  role_arn = local.data_access_role_arn
}

data "velodb_aws_data_access_policy" "data_access" {
  bucket_name = var.bucket_name
  role_arn    = local.data_access_role_arn
  tde_kms_arn = var.tde_kms_arn
}

data "velodb_aws_assume_role_policy" "deployment" {
  principal_arn = data.velodb_byoc_prerequisites.aws.deployment_assumer_role_arn
  external_id   = data.velodb_byoc_prerequisites.aws.external_id
}

data "velodb_aws_crossaccount_policy" "deployment" {
  bucket_name         = var.bucket_name
  data_credential_arn = local.data_access_profile_arn
}

resource "aws_iam_role" "data_access" {
  name               = local.data_access_role_name
  assume_role_policy = data.velodb_aws_data_access_assume_role_policy.data_access.json
}

resource "aws_iam_policy" "data_access" {
  name   = "${local.data_access_role_name}-policy"
  policy = data.velodb_aws_data_access_policy.data_access.json
}

resource "aws_iam_role_policy_attachment" "data_access" {
  role       = aws_iam_role.data_access.name
  policy_arn = aws_iam_policy.data_access.arn
}

resource "aws_iam_instance_profile" "data_access" {
  name = local.data_access_role_name
  role = aws_iam_role.data_access.name
}

resource "aws_iam_role" "deployment" {
  name               = local.deployment_role_name
  assume_role_policy = data.velodb_aws_assume_role_policy.deployment.json
}

resource "aws_iam_policy" "deployment" {
  name   = "${local.deployment_role_name}-policy"
  policy = data.velodb_aws_crossaccount_policy.deployment.json
}

resource "aws_iam_role_policy_attachment" "deployment" {
  role       = aws_iam_role.deployment.name
  policy_arn = aws_iam_policy.deployment.arn
}

check "generated_policies" {
  assert {
    condition     = length(jsondecode(data.velodb_aws_assume_role_policy.deployment.json).Statement) == 1
    error_message = "Deployment trust policy must contain one statement."
  }
  assert {
    condition     = length(jsondecode(data.velodb_aws_crossaccount_policy.deployment.json).Statement) == 17
    error_message = "Deployment permissions policy must contain 17 statements."
  }
  assert {
    condition     = length(jsondecode(data.velodb_aws_data_access_assume_role_policy.data_access.json).Statement) == 2
    error_message = "Data-access trust policy must contain two statements."
  }
  assert {
    condition     = length(jsondecode(data.velodb_aws_data_access_policy.data_access.json).Statement) == (var.tde_kms_arn == null ? 3 : 4)
    error_message = "Data-access permissions policy has an unexpected statement count."
  }
}

output "live_test" {
  value = {
    aws_account_id       = local.account_id
    external_id          = data.velodb_byoc_prerequisites.aws.external_id
    deployment_principal = data.velodb_byoc_prerequisites.aws.deployment_assumer_role_arn
    data_access_role_arn = aws_iam_role.data_access.arn
    instance_profile_arn = aws_iam_instance_profile.data_access.arn
    deployment_role_arn  = aws_iam_role.deployment.arn
  }
}
