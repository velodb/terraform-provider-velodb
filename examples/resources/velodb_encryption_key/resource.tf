# Register a KMS key for transparent data encryption (TDE) of warehouse data.
# The KMS key policy must grant the data-access role the required permissions.
# Generate it with the velodb_aws_kms_key_policy data source.
data "velodb_aws_kms_key_policy" "tde" {
  use_tde       = true
  data_role_arn = var.data_access_role_arn
}

resource "aws_kms_key" "tde" {
  description             = "VeloDB warehouse TDE key"
  deletion_window_in_days = 30
  enable_key_rotation     = true
  policy                  = data.velodb_aws_kms_key_policy.tde.json
}

resource "velodb_encryption_key" "tde" {
  cloud_provider = "aws"
  name           = "production-tde"
  key_arn        = aws_kms_key.tde.arn
  use_tde        = true
}

# Register an existing KMS key for EBS volume encryption (bring your own key).
resource "velodb_encryption_key" "ebs" {
  cloud_provider = "aws"
  name           = "production-ebs"
  key_arn        = var.ebs_kms_key_arn
  use_ebs        = true
}

# Reference the registered keys when creating a warehouse.
resource "velodb_warehouse" "this" {
  # ... other BYOC configuration ...
  tde_encryption_key_id = velodb_encryption_key.tde.id
  ebs_encryption_key_id = velodb_encryption_key.ebs.id
}

variable "data_access_role_arn" {
  description = "AWS IAM data-access role ARN used by warehouse instances."
  type        = string
}

variable "ebs_kms_key_arn" {
  description = "ARN of an existing AWS KMS key to use for EBS encryption."
  type        = string
}
