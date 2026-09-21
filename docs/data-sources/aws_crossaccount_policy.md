---
page_title: "velodb_aws_crossaccount_policy Data Source - velodb"
description: Generates the AWS deployment-role permissions required by VeloDB Cloud.
---

# velodb_aws_crossaccount_policy (Data Source)

Generates policy JSON only. The data credential is an instance-profile ARN, not a role ARN.

```terraform
data "velodb_aws_crossaccount_policy" "deployment" {
  bucket_name         = aws_s3_bucket.data.bucket
  data_credential_arn = aws_iam_instance_profile.data.arn
}

resource "aws_iam_policy" "deployment" {
  name   = "VeloDBCrossAccountRolePolicy"
  policy = data.velodb_aws_crossaccount_policy.deployment.json
}
```

## Schema

Required:

- `bucket_name` — S3 bucket used by the warehouse.
- `data_credential_arn` — Commercial AWS IAM instance-profile ARN used by warehouse instances.

Read-only:

- `json` — AWS IAM policy document.
