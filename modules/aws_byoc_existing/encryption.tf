# Warehouse encryption keys (TDE and EBS).
#
# Each key is independent and supports both "create new" and "bring existing":
#   - create_tde_encryption_key / create_ebs_encryption_key create a new AWS KMS
#     key whose key policy grants the existing data-access and deployment roles
#     the access VeloDB requires.
#   - tde_kms_key_arn / ebs_kms_key_arn register an existing KMS key instead. For
#     an existing key, ensure its key policy already grants those roles access
#     (the velodb_aws_kms_key_policy data source generates the required policy).
# When neither is set for a use, that encryption key is not configured.

locals {
  data_access_role_arn = data.aws_iam_instance_profile.data_access.role_arn
  deployment_role_arn  = var.deployment_credential_arn

  tde_encryption_key_arn = var.create_tde_encryption_key ? one(aws_kms_key.tde[*].arn) : var.tde_kms_key_arn
  ebs_encryption_key_arn = var.create_ebs_encryption_key ? one(aws_kms_key.ebs[*].arn) : var.ebs_kms_key_arn

  tde_encryption_enabled = local.tde_encryption_key_arn != null
  ebs_encryption_enabled = local.ebs_encryption_key_arn != null
}

resource "terraform_data" "encryption_guard" {
  lifecycle {
    precondition {
      condition     = !(var.create_tde_encryption_key && var.tde_kms_key_arn != null)
      error_message = "Set only one of create_tde_encryption_key or tde_kms_key_arn."
    }
    precondition {
      condition     = !(var.create_ebs_encryption_key && var.ebs_kms_key_arn != null)
      error_message = "Set only one of create_ebs_encryption_key or ebs_kms_key_arn."
    }
  }
}

# --- New KMS keys -----------------------------------------------------------

data "velodb_aws_kms_key_policy" "tde" {
  count = var.create_tde_encryption_key ? 1 : 0

  use_tde       = true
  data_role_arn = local.data_access_role_arn
}

resource "aws_kms_key" "tde" {
  count = var.create_tde_encryption_key ? 1 : 0

  description             = "${var.name_prefix} VeloDB warehouse TDE key"
  deletion_window_in_days = var.kms_key_deletion_window_in_days
  enable_key_rotation     = true
  policy                  = data.velodb_aws_kms_key_policy.tde[0].json
  tags                    = local.tags
}

resource "aws_kms_alias" "tde" {
  count = var.create_tde_encryption_key ? 1 : 0

  name          = "alias/${var.name_prefix}-tde"
  target_key_id = aws_kms_key.tde[0].key_id
}

data "velodb_aws_kms_key_policy" "ebs" {
  count = var.create_ebs_encryption_key ? 1 : 0

  use_ebs             = true
  deployment_role_arn = local.deployment_role_arn
}

resource "aws_kms_key" "ebs" {
  count = var.create_ebs_encryption_key ? 1 : 0

  description             = "${var.name_prefix} VeloDB warehouse EBS key"
  deletion_window_in_days = var.kms_key_deletion_window_in_days
  enable_key_rotation     = true
  policy                  = data.velodb_aws_kms_key_policy.ebs[0].json
  tags                    = local.tags
}

resource "aws_kms_alias" "ebs" {
  count = var.create_ebs_encryption_key ? 1 : 0

  name          = "alias/${var.name_prefix}-ebs"
  target_key_id = aws_kms_key.ebs[0].key_id
}

# --- VeloDB encryption key registrations ------------------------------------

resource "velodb_encryption_key" "tde" {
  count = local.tde_encryption_enabled ? 1 : 0

  cloud_provider = "aws"
  name           = "${var.name_prefix}-tde"
  key_arn        = local.tde_encryption_key_arn
  use_tde        = true
}

resource "velodb_encryption_key" "ebs" {
  count = local.ebs_encryption_enabled ? 1 : 0

  cloud_provider = "aws"
  name           = "${var.name_prefix}-ebs"
  key_arn        = local.ebs_encryption_key_arn
  use_ebs        = true
}
