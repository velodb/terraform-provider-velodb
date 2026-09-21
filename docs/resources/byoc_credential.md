---
page_title: "velodb_byoc_credential Resource - velodb"
subcategory: ""
description: |-
  Registers AWS storage and deployment credentials for an advanced BYOC warehouse.
---

# velodb_byoc_credential (Resource)

Registers the S3 bucket, instance-profile ARN, and cross-account deployment-role
ARN used by an advanced AWS BYOC warehouse. VeloDB validates these resources
during creation.

## Example Usage

```terraform
resource "velodb_byoc_credential" "aws" {
  cloud_provider            = "aws"
  name                      = "production-credential"
  region                    = "us-east-1"
  bucket_name               = var.bucket_name
  data_credential_arn       = var.data_credential_arn
  deployment_credential_arn = var.deployment_credential_arn

  depends_on = [
    aws_iam_role_policy_attachment.data_access_attach,
    aws_iam_role_policy_attachment.deployment_attach,
  ]
}
```

The explicit dependencies ensure AWS has attached both policies before VeloDB
validates the credential.

## Schema

### Required

- `bucket_name` (String) S3 bucket used by the warehouse. Changing this replaces the resource.
- `cloud_provider` (String) Cloud provider. Only `aws` is supported. Changing this replaces the resource.
- `data_credential_arn` (String) AWS instance-profile ARN used to access warehouse data. Changing this replaces the resource.
- `deployment_credential_arn` (String) AWS IAM role ARN used by VeloDB to deploy warehouse infrastructure. Changing this replaces the resource.
- `name` (String) Credential configuration name. Changing this replaces the resource.
- `region` (String) AWS region. Changing this replaces the resource.

### Read-Only

- `created_at` (String) Creation time in RFC 3339 format.
- `external_id` (String) External ID validated by VeloDB.
- `id` (Number) VeloDB credential configuration ID.
- `updated_at` (String) Last update time in RFC 3339 format.
- `warehouse_count` (Number) Number of warehouses using this credential configuration.
- `warehouse_ids` (List of String) Warehouses using this credential configuration.

## Import

```shell
terraform import velodb_byoc_credential.example aws/<credential_id>
```
