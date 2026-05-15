# Get connection endpoints for a warehouse
data "velodb_warehouse_connections" "prod" {
  warehouse_id = "ALBJ15F0"
}

output "jdbc_url" {
  value = [for ep in data.velodb_warehouse_connections.prod.public_endpoints : ep.url if ep.protocol == "jdbc"][0]
}

output "http_url" {
  value = [for ep in data.velodb_warehouse_connections.prod.public_endpoints : ep.url if ep.protocol == "http"][0]
}
