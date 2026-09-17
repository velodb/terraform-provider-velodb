data "velodb_byoc_prerequisites" "aws" {
  cloud_provider = "aws"
  region         = "us-east-1"
}

output "external_id" {
  value = data.velodb_byoc_prerequisites.aws.external_id
}

output "deployment_assumer_role_arn" {
  value = data.velodb_byoc_prerequisites.aws.deployment_assumer_role_arn
}

output "endpoint_service_name" {
  value = data.velodb_byoc_prerequisites.aws.endpoint_service_name
}
