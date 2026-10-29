# Freeze creation inputs so edits fail before replacing infrastructure used by
# the warehouse. JSON preserves nulls and heterogeneous input types.
resource "terraform_data" "warehouse_infrastructure_guard" {
  for_each = {
    initial_cluster_capacity = jsonencode({
      compute_vcpu = local.initial_compute_vcpu
      cache_gb     = local.initial_cache_gb
      node_vcpu    = var.node_vcpu
      node_count   = var.node_count
      node_cache   = var.node_cache
      cache_type   = var.cache_type
      ratio        = var.ratio
    })
    region                    = jsonencode(var.region)
    bucket_name               = jsonencode(var.bucket_name)
    create_tde_encryption_key = jsonencode(var.create_tde_encryption_key)
    create_ebs_encryption_key = jsonencode(var.create_ebs_encryption_key)
    tde_kms_key_arn           = jsonencode(var.tde_kms_key_arn)
    ebs_kms_key_arn           = jsonencode(var.ebs_kms_key_arn)
    vpc_cidr                  = jsonencode(var.vpc_cidr)
    zones                     = jsonencode(var.zones)
    subnet_cidrs              = jsonencode(var.subnet_cidrs)
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
