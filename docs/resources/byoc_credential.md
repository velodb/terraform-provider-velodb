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

  # The warehouse and network both reference this credential, so it is the last
  # VeloDB resource destroyed. Anchor every underlying AWS resource here.
  depends_on = [
    aws_iam_role_policy_attachment.data_access,
    aws_iam_role_policy_attachment.deployment,
    aws_vpc_endpoint.s3,
    aws_vpc_endpoint.velodb,
    aws_route.private_nat,
    aws_vpc_security_group_ingress_rule.warehouse_self,
    aws_vpc_security_group_egress_rule.warehouse_all,
    aws_vpc_security_group_ingress_rule.endpoint_https,
    aws_vpc_security_group_egress_rule.endpoint_all,
  ]
}
```

Anchoring the AWS IAM, storage, and network resources on the credential serves
both directions of the graph. On create, they are ready before VeloDB validates
the credential. On destroy, Terraform reverses the graph, so they are removed
only after the credential -- and therefore after the warehouse has finished
deleting. Without these dependencies Terraform is free to delete resources such
as the S3 gateway endpoint, the PrivateLink endpoint, the NAT default route, and
the security-group rules in parallel with the warehouse, which can strand the
backend deletion without S3 or network access and leave the warehouse stuck
deleting. List every AWS resource the running warehouse relies on.

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
