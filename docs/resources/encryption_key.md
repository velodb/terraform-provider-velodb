---
page_title: "velodb_encryption_key Resource - velodb"
subcategory: ""
description: |-
  Registers a customer-provided AWS KMS key with VeloDB for warehouse TDE and/or EBS encryption.
---

# velodb_encryption_key (Resource)

Registers a customer-provided AWS KMS key so it can be used as a warehouse
encryption key. A registered key is referenced by ID from `velodb_warehouse`:

- `use_tde` keys are referenced via `tde_encryption_key_id` and encrypt warehouse
  data (transparent data encryption).
- `use_ebs` keys are referenced via `ebs_encryption_key_id` and encrypt the
  warehouse's EBS volumes.

A single key may enable both uses, or you can register separate keys per use. At
least one of `use_tde` or `use_ebs` must be `true`.

The KMS key policy must grant VeloDB the access it requires. Generate the correct
key policy with the [`velodb_aws_kms_key_policy`](../data-sources/aws_kms_key_policy.md)
data source. The [`aws_byoc`](https://github.com/velodb/terraform-provider-velodb/tree/main/modules/aws_byoc)
and `aws_byoc_existing` modules can create and register these keys for you.

## Example Usage

```terraform
data "velodb_aws_kms_key_policy" "tde" {
  use_tde       = true
  data_role_arn = var.data_access_role_arn
}

resource "aws_kms_key" "tde" {
  description             = "VeloDB warehouse TDE key"
  deletion_window_in_days = 30
  enable_key_rotation     = true
  policy                  = data.velodb_aws_kms_key_policy.tde.json
}

resource "velodb_encryption_key" "tde" {
  cloud_provider = "aws"
  name           = "production-tde"
  key_arn        = aws_kms_key.tde.arn
  use_tde        = true
}
```

## Schema

### Required

- `cloud_provider` (String) Cloud provider. Only `aws` is supported. Changing this replaces the resource.
- `key_arn` (String) AWS KMS key ARN. Changing this replaces the resource.
- `name` (String) Encryption key configuration name, unique within the organization. Changing this replaces the resource.

### Optional

- `use_ebs` (Boolean) Allow this key to encrypt warehouse EBS volumes. Defaults to `false`. Changing this replaces the resource. At least one of `use_tde` or `use_ebs` must be `true`.
- `use_tde` (Boolean) Allow this key to be used for transparent data encryption (TDE) of warehouse data. Defaults to `false`. Changing this replaces the resource. At least one of `use_tde` or `use_ebs` must be `true`.

### Read-Only

- `created_at` (String) Creation time.
- `id` (Number) VeloDB encryption key configuration ID.
- `region` (String) AWS region derived from the KMS key ARN.
- `updated_at` (String) Last update time.
- `warehouse_count` (Number) Number of warehouses using this encryption key.
- `warehouse_ids` (List of String) Warehouses using this encryption key.

## Import

```shell
terraform import velodb_encryption_key.example aws/<encryption_key_id>
```
