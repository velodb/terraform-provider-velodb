---
page_title: "velodb_byoc_network Resource - velodb"
subcategory: ""
description: |-
  Registers an AWS VPC network configuration for an advanced BYOC warehouse.
---

# velodb_byoc_network (Resource)

Registers the subnets, security group, and optional VPC endpoint used by an
advanced AWS BYOC warehouse. Use one zone mapping for single-AZ deployment or
three unique mappings for multi-AZ deployment.

## Example Usage

```terraform
resource "velodb_byoc_network" "aws" {
  cloud_provider    = "aws"
  name              = "production-network"
  credential_id     = velodb_byoc_credential.aws.id
  security_group_id = var.security_group_id
  endpoint_id       = var.endpoint_id

  zone_mappings = [{
    zone_id   = "us-east-1a"
    subnet_id = var.subnet_id
  }]
}
```

## Schema

### Required

- `cloud_provider` (String) Cloud provider. Only `aws` is supported. Changing this replaces the resource.
- `credential_id` (Number) Credential configuration used by this network. Changing this replaces the resource.
- `name` (String) Network configuration name. Changing this replaces the resource.
- `security_group_id` (String) AWS security group ID for the warehouse. Changing this replaces the resource.
- `zone_mappings` (Attributes List) Exactly one or three unique zone-to-subnet mappings. Changing this replaces the resource.

### Optional

- `endpoint_id` (String) AWS VPC endpoint ID. Changing this replaces the resource.

### Read-Only

- `created_at` (String) Creation time in RFC 3339 format.
- `id` (Number) VeloDB network configuration ID.
- `region` (String) AWS region derived from the registered network.
- `updated_at` (String) Last update time in RFC 3339 format.
- `vpc_id` (String) AWS VPC ID derived from the registered network.
- `warehouse_count` (Number) Number of warehouses using this network configuration.
- `warehouse_ids` (List of String) Warehouses using this network configuration.

### Nested Schema for `zone_mappings`

Required:

- `subnet_id` (String) Private subnet ID in the Availability Zone.
- `zone_id` (String) AWS Availability Zone name, such as `us-east-1a`.

## Import

Import requires both IDs because the network API does not return its credential
configuration ID:

```shell
terraform import velodb_byoc_network.example aws/<network_config_id>/<credential_id>
```
