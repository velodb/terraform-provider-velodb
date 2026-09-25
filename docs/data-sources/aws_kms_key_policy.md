---
page_title: "velodb_aws_kms_key_policy Data Source - velodb"
description: Generates the KMS key policy VeloDB requires on a warehouse encryption key.
---

# velodb_aws_kms_key_policy (Data Source)

Generates the resource-based KMS key policy that VeloDB Cloud requires on the
customer-provided KMS key registered as a
[`velodb_encryption_key`](../resources/encryption_key.md). The policy delegates to
the account's IAM policies and grants:

- the data-access role transparent data encryption (TDE) access when `use_tde` is
  set, and
- the deployment role EBS volume encryption access (scoped to EC2 via a
  `kms:ViaService` condition) when `use_ebs` is set.

At least one of `use_tde` or `use_ebs` must be set; each enabled use requires its
corresponding role ARN.

```terraform
data "velodb_aws_kms_key_policy" "tde" {
  use_tde       = true
  data_role_arn = aws_iam_role.data_access.arn
}

resource "aws_kms_key" "tde" {
  description = "VeloDB warehouse TDE key"
  policy      = data.velodb_aws_kms_key_policy.tde.json
}
```

## Schema

Optional:

- `use_tde` — Grant the data-access role TDE permissions. Requires `data_role_arn`.
- `use_ebs` — Grant the deployment role EBS encryption permissions. Requires `deployment_role_arn`.
- `data_role_arn` — Commercial AWS IAM data-access role ARN. Required when `use_tde` is true.
- `deployment_role_arn` — Commercial AWS IAM deployment role ARN. Required when `use_ebs` is true.

Read-only:

- `json` — AWS KMS key policy document.
