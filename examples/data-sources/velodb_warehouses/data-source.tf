# List all SaaS warehouses in us-east-1
data "velodb_warehouses" "us_saas" {
  cloud_provider  = "aws"
  region          = "us-east-1"
  deployment_mode = "SaaS"
}

output "warehouse_count" {
  value = data.velodb_warehouses.us_saas.total
}

output "warehouse_names" {
  value = [for wh in data.velodb_warehouses.us_saas.warehouses : wh.name]
}

output "warehouse_placement_zones" {
  value = {
    for wh in data.velodb_warehouses.us_saas.warehouses :
    wh.warehouse_id => keys(coalesce(wh.zone_map, tomap({})))
  }
}
