# List all SaaS warehouses in cn-beijing
data "velodb_warehouses" "beijing_saas" {
  cloud_provider  = "aliyun"
  region          = "cn-beijing"
  deployment_mode = "SaaS"
}

output "warehouse_count" {
  value = data.velodb_warehouses.beijing_saas.total
}

output "warehouse_names" {
  value = [for wh in data.velodb_warehouses.beijing_saas.warehouses : wh.name]
}
