output "aws_account_id" {
  value = data.aws_caller_identity.current.account_id
}

output "zones" {
  value = var.zones
}

output "bucket_name" {
  value = aws_s3_bucket.data.bucket
}

output "vpc_id" {
  value = aws_vpc.this.id
}

output "private_subnet_ids" {
  value = local.private_subnet_ids
}

output "endpoint_id" {
  value = aws_vpc_endpoint.velodb.id
}

output "credential_id" {
  value = velodb_byoc_credential.this.id
}

output "network_config_id" {
  value = velodb_byoc_network.this.id
}

output "tde_encryption_key_id" {
  description = "VeloDB encryption key ID used for warehouse TDE, or null when not configured."
  value       = one(velodb_encryption_key.tde[*].id)
}

output "ebs_encryption_key_id" {
  description = "VeloDB encryption key ID used for warehouse EBS encryption, or null when not configured."
  value       = one(velodb_encryption_key.ebs[*].id)
}

output "tde_kms_key_arn" {
  description = "ARN of the KMS key registered for warehouse TDE, or null when not configured."
  value       = local.tde_encryption_key_arn
}

output "ebs_kms_key_arn" {
  description = "ARN of the KMS key registered for warehouse EBS encryption, or null when not configured."
  value       = local.ebs_encryption_key_arn
}

output "warehouse_id" {
  value = velodb_warehouse.this.id
}

output "warehouse_status" {
  value = velodb_warehouse.this.status
}

output "initial_cluster_id" {
  value = velodb_warehouse.this.initial_cluster_id
}

output "additional_cluster_ids" {
  description = "Additional cluster IDs keyed by their stable Terraform identifiers."
  value       = { for key, cluster in velodb_cluster.additional : key => cluster.id }
}
