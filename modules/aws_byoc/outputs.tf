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

output "warehouse_id" {
  value = velodb_warehouse.this.id
}

output "warehouse_status" {
  value = velodb_warehouse.this.status
}

output "initial_cluster_id" {
  value = velodb_warehouse.this.initial_cluster_id
}

output "second_cluster_id" {
  description = "Deprecated output for create_second_cluster. Use additional_cluster_ids instead."
  value       = try(velodb_cluster.second[0].id, null)
}

output "additional_cluster_ids" {
  description = "Additional cluster IDs keyed by their stable Terraform identifiers."
  value       = { for key, cluster in velodb_cluster.additional : key => cluster.id }
}
