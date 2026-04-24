# List running COMPUTE clusters in a warehouse
data "velodb_clusters" "running_compute" {
  warehouse_id = "ALBJRXRG"
  status       = "Running"
  cluster_type = "COMPUTE"
}

output "cluster_names" {
  value = [for cl in data.velodb_clusters.running_compute.clusters : cl.name]
}
