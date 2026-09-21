---
page_title: "velodb_aws_assume_role_policy Data Source - velodb"
description: Generates the AWS deployment-role trust policy for VeloDB Cloud.
---

# velodb_aws_assume_role_policy (Data Source)

Generates policy JSON only. Use the AWS provider to create the IAM role.

```terraform
data "velodb_byoc_prerequisites" "aws" {
  cloud_provider = "aws"
  region         = var.region
}

data "velodb_aws_assume_role_policy" "deployment" {
  principal_arn = data.velodb_byoc_prerequisites.aws.deployment_assumer_role_arn
  external_id   = data.velodb_byoc_prerequisites.aws.external_id
}

resource "aws_iam_role" "deployment" {
  name               = "VeloDBCrossAccountRole"
  assume_role_policy = data.velodb_aws_assume_role_policy.deployment.json
}
```

## Schema

Required:

- `principal_arn` — VeloDB deployment-assumer role ARN from `velodb_byoc_prerequisites`.
- `external_id` — Organization external ID from `velodb_byoc_prerequisites`.

Read-only:

- `json` — AWS IAM policy document.
