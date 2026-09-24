output "bucket_name" {
  value = var.bucket_name
}

output "vpc_id" {
  value = var.vpc_id
}

output "subnet_ids_by_zone" {
  value = var.subnet_ids_by_zone
}

output "endpoint_id" {
  value = var.endpoint_id
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

output "additional_cluster_ids" {
  description = "Additional cluster IDs keyed by their stable Terraform identifiers."
  value       = { for key, cluster in velodb_cluster.additional : key => cluster.id }
}

output "aws_resources_managed" {
  description = "Always false: this module reads but does not manage existing AWS resources."
  value       = false
}
