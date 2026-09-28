# Freeze creation inputs so edits fail before replacing infrastructure used by
# the warehouse. JSON preserves nulls and heterogeneous input types.
resource "terraform_data" "warehouse_infrastructure_guard" {
  for_each = {
    region                    = jsonencode(var.region)
    bucket_name               = jsonencode(var.bucket_name)
    create_tde_encryption_key = jsonencode(var.create_tde_encryption_key)
    create_ebs_encryption_key = jsonencode(var.create_ebs_encryption_key)
    tde_kms_key_arn           = jsonencode(var.tde_kms_key_arn)
    ebs_kms_key_arn           = jsonencode(var.ebs_kms_key_arn)
    vpc_id                    = jsonencode(var.vpc_id)
    subnet_ids_by_zone        = jsonencode(var.subnet_ids_by_zone)
    security_group_id         = jsonencode(var.security_group_id)
    endpoint_id               = jsonencode(var.endpoint_id)
    data_credential_arn       = jsonencode(var.data_credential_arn)
    deployment_credential_arn = jsonencode(var.deployment_credential_arn)
  }
  input = each.value

  lifecycle {
    ignore_changes = [input]
    postcondition {
      condition     = self.output == each.value
      error_message = "${each.key} cannot be changed after creation because it is used by the warehouse. Restore the original value; provision a separate warehouse for a migration."
    }
  }
}
