---
page_title: "velodb_aws_data_access_assume_role_policy Data Source - velodb"
description: Generates the AWS data-access role trust policy.
---

# velodb_aws_data_access_assume_role_policy (Data Source)

Generates the EC2 and condition-scoped self-assumption trust policy for the warehouse data-access role.

```terraform
data "aws_caller_identity" "current" {}

locals {
  data_role_arn = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/VeloDBDataStorageAccessRole"
}

data "velodb_aws_data_access_assume_role_policy" "data" {
  role_arn = local.data_role_arn
}

resource "aws_iam_role" "data" {
  name               = "VeloDBDataStorageAccessRole"
  assume_role_policy = data.velodb_aws_data_access_assume_role_policy.data.json
}
```

## Schema

Required:

- `role_arn` — Commercial AWS IAM data-access role ARN.

Read-only:

- `json` — AWS IAM policy document.
