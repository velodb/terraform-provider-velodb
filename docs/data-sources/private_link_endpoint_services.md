---
page_title: "velodb_private_link_endpoint_services Data Source - velodb"
subcategory: ""
description: |-
  Lists outbound PrivateLink endpoint services registered with VeloDB Cloud.
---

# velodb_private_link_endpoint_services (Data Source)

Use the *velodb_private_link_endpoint_services* data source to read outbound PrivateLink endpoint services that are already registered with VeloDB Cloud. This is the read-only companion to the [`velodb_private_link_endpoint_service`](../resources/private_link_endpoint_service.md) resource.

## Example Usage

```terraform
data "velodb_private_link_endpoint_services" "aws" {
  cloud_provider = "aws"
  region         = "us-east-1"
}

output "outbound_services" {
  value = data.velodb_private_link_endpoint_services.aws.services
}
```

### Find One Service

```terraform
data "velodb_private_link_endpoint_services" "corp_api" {
  cloud_provider      = "aws"
  region              = "us-east-1"
  endpoint_service_id = "vpce-svc-0abc123def456"
}

output "connected" {
  value = data.velodb_private_link_endpoint_services.corp_api.services[0].connected
}
```

## Schema

### Optional

- `cloud_provider` (String) Cloud provider filter, such as `aws` or `aliyun`.
- `endpoint_service_id` (String) Exact cloud-side endpoint service ID filter.
- `endpoint_service_name` (String) Exact cloud-side endpoint service name filter.
- `region` (String) Cloud region filter.

### Read-Only

- `total` (Number) Total number of matching endpoint services.
- `services` (Attributes List) List of matching outbound PrivateLink endpoint services. (see [below for nested schema](#nestedatt--services))

<a id="nestedatt--services"></a>
### Nested Schema for `services`

Read-Only:

- `cloud_provider` (String) Cloud provider.
- `connected` (Boolean) Whether the service is currently connected.
- `created_at` (String) Registration time in RFC 3339 format.
- `description` (String) Service description.
- `endpoint_service_id` (String) Cloud-side endpoint service ID.
- `endpoint_service_name` (String) Cloud-side endpoint service name.
- `endpoints` (Attributes List) Private endpoints connected to this outbound service. (see [below for nested schema](#nestedatt--services--endpoints))
- `provider_account_id` (String) Cloud account ID that owns the endpoint service.
- `region` (String) Cloud region.
- `zone` (String) Availability zone associated with the endpoint service when known.

<a id="nestedatt--services--endpoints"></a>
### Nested Schema for `services.endpoints`

Read-Only:

- `created_at` (String) Endpoint creation time in RFC 3339 format.
- `domain` (String) Endpoint DNS name.
- `endpoint_id` (String) Private endpoint ID.
- `endpoint_name` (String) Private endpoint name.
- `status` (String) Endpoint status returned by the API.
