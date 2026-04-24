# Get connection endpoints for a warehouse
data "velodb_warehouse_connections" "prod" {
  warehouse_id = "ALBJ15F0"
}

output "jdbc_url" {
  value = "jdbc:mysql://${data.velodb_warehouse_connections.prod.clusters[0].public_endpoint}:${data.velodb_warehouse_connections.prod.clusters[0].jdbc_port}"
}

output "http_url" {
  value = "http://${data.velodb_warehouse_connections.prod.clusters[0].public_endpoint}:${data.velodb_warehouse_connections.prod.clusters[0].http_port}"
}
