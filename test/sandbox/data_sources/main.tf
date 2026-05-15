terraform {
  required_providers {
    velodb = {
      source = "velodb/velodb"
    }
  }
}

provider "velodb" {
  host    = "sandbox-api.velodb.io"
  api_key = var.api_key
}

variable "api_key" {
  type      = string
  sensitive = true
}

variable "warehouse_id" {
  type = string
}

variable "warehouse_name" {
  type = string
}

variable "cluster_name" {
  type = string
}

data "velodb_warehouses" "by_name" {
  name            = var.warehouse_name
  cloud_provider  = "aws"
  region          = "us-east-1"
  deployment_mode = "SaaS"
}

data "velodb_warehouses" "by_keyword" {
  keyword         = var.warehouse_name
  cloud_provider  = "aws"
  region          = "us-east-1"
  deployment_mode = "SaaS"
}

data "velodb_clusters" "by_name" {
  warehouse_id = var.warehouse_id
  cluster_name = var.cluster_name
  cluster_type = "COMPUTE"
  status       = "Running"
}

data "velodb_clusters" "by_keyword" {
  warehouse_id = var.warehouse_id
  keyword      = var.cluster_name
}

data "velodb_warehouse_connections" "current" {
  warehouse_id = var.warehouse_id
}

data "velodb_warehouse_versions" "current" {
  warehouse_id = var.warehouse_id
}

output "warehouse_name_total" {
  value = data.velodb_warehouses.by_name.total
}

output "warehouse_keyword_total" {
  value = data.velodb_warehouses.by_keyword.total
}

output "warehouse_endpoint_service_name" {
  value = try(data.velodb_warehouses.by_name.warehouses[0].endpoint_service_name, "")
}

output "cluster_name_total" {
  value = data.velodb_clusters.by_name.total
}

output "cluster_keyword_total" {
  value = data.velodb_clusters.by_keyword.total
}

output "cluster_auto_pause_count" {
  value = try(length(data.velodb_clusters.by_name.clusters[0].auto_pause), 0)
}

output "connections_public_count" {
  value = length(data.velodb_warehouse_connections.current.public_endpoints)
}

output "connections_private_count" {
  value = length(data.velodb_warehouse_connections.current.private_endpoints)
}

output "connections_cluster_count" {
  value = length(data.velodb_warehouse_connections.current.compute_clusters)
}

output "version_default_id" {
  value = data.velodb_warehouse_versions.current.default_id
}
