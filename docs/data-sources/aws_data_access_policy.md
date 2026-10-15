---
page_title: "velodb_aws_data_access_policy Data Source - velodb"
description: Generates AWS data-access permissions for a VeloDB warehouse.
---

# velodb_aws_data_access_policy (Data Source)

Generates the S3, self-assumption, and optional TDE KMS permissions for the warehouse data-access role.

```terraform
data "velodb_aws_data_access_policy" "data" {
  bucket_name = aws_s3_bucket.data.bucket
  role_arn    = aws_iam_role.data.arn
  tde_kms_arn = aws_kms_key.tde.arn
}

resource "aws_iam_policy" "data" {
  name   = "VeloDBDataStorageAccessRolePolicy"
  policy = data.velodb_aws_data_access_policy.data.json
}
```

## Schema

Required:

- `bucket_name` — S3 bucket used by the warehouse.
- `role_arn` — Commercial AWS IAM data-access role ARN.

Optional:

- `tde_kms_arn` — Commercial AWS KMS key ARN used for transparent data encryption.

Read-only:

- `json` — AWS IAM policy document.

Bucket names are checked during configuration validation: use 3–63 lowercase
letters, numbers, periods, or hyphens, starting and ending with a letter or
number. Adjacent periods, IP address formats, and AWS reserved prefixes/suffixes
are rejected. Unknown values are checked once resolved. See the
[AWS naming rules](https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html).
