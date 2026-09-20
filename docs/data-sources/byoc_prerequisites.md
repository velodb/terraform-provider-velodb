---
page_title: "velodb_byoc_prerequisites Data Source - velodb"
subcategory: ""
description: |-
  Discovers organization and regional prerequisites for an AWS BYOC warehouse.
---

# velodb_byoc_prerequisites (Data Source)

Use this data source before creating AWS resources for an advanced BYOC
warehouse. It returns the organization external ID, the VeloDB role ARN to
trust, the regional PrivateLink service, and supported Availability Zones.

The API key must be allowed to read the organization profile.

## Example Usage

```terraform
data "velodb_byoc_prerequisites" "aws" {
  cloud_provider = "aws"
  region         = "us-east-1"
}

output "external_id" {
  value = data.velodb_byoc_prerequisites.aws.external_id
}

output "deployment_assumer_role_arn" {
  value = data.velodb_byoc_prerequisites.aws.deployment_assumer_role_arn
}
```

## Schema

### Required

- `cloud_provider` (String) Cloud provider. Only `aws` is supported.
- `region` (String) AWS region for the BYOC warehouse.

### Read-Only

- `deployment_assumer_role_arn` (String) VeloDB AWS principal ARN for the deployment-role trust policy.
- `endpoint_service_id` (String) VeloDB PrivateLink endpoint service ID for the region.
- `endpoint_service_name` (String) VeloDB PrivateLink endpoint service name for the region.
- `external_id` (String) Organization external ID for the deployment-role trust policy.
- `multi_az_supported` (Boolean) Whether the region supports multi-AZ deployment.
- `zones` (Attributes List) Supported Availability Zones.

### Nested Schema for `zones`

Read-Only:

- `display_name` (String) Display name returned by VeloDB.
- `zone` (String) AWS Availability Zone name.
