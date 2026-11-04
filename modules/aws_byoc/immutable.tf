# Reject changes to warehouse creation inputs; JSON preserves nulls and types.
resource "terraform_data" "warehouse_infrastructure_guard" {
  for_each = {
    initial_cluster_capacity = jsonencode({
      compute_vcpu       = local.initial_compute_vcpu
      cache_gb           = local.initial_cache_gb
      node_vcpu          = var.node_vcpu
      node_count         = var.node_count
      node_cache         = var.node_cache
      cache_storage_type = var.cache_storage_type
      ratio              = var.ratio
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
      # Keep the original snapshot address while allowing capacity updates.
      condition = each.key == "initial_cluster_capacity" ? alltrue([
        for key in ["cache_storage_type", "ratio"] : try(jsondecode(self.output)[key], null) == jsondecode(each.value)[key]
      ]) : self.output == each.value
      error_message = each.key == "initial_cluster_capacity" ? "Initial-cluster cache_storage_type and ratio cannot be changed after creation. Restore their original values." : "${each.key} cannot be changed after creation because it is used by the warehouse. Restore the original value; provision a separate warehouse for a migration."
    }
  }
}
