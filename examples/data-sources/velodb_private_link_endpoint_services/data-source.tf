data "velodb_private_link_endpoint_services" "aws" {
  cloud_provider = "aws"
  region         = "us-east-1"
}

output "outbound_endpoint_services" {
  value = data.velodb_private_link_endpoint_services.aws.services
}

output "connected_outbound_service_ids" {
  value = [
    for svc in data.velodb_private_link_endpoint_services.aws.services :
    svc.endpoint_service_id
    if svc.connected
  ]
}
