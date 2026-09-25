# TDE key policy: grants the data-access role transparent data encryption access.
data "velodb_aws_kms_key_policy" "tde" {
  use_tde       = true
  data_role_arn = aws_iam_role.data_access.arn
}

# EBS key policy: grants the deployment role EBS volume encryption access.
data "velodb_aws_kms_key_policy" "ebs" {
  use_ebs             = true
  deployment_role_arn = aws_iam_role.deployment.arn
}

resource "aws_kms_key" "tde" {
  description = "VeloDB warehouse TDE key"
  policy      = data.velodb_aws_kms_key_policy.tde.json
}
