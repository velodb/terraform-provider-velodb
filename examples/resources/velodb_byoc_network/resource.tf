resource "velodb_byoc_network" "aws" {
  cloud_provider    = "aws"
  name              = "production-network"
  credential_id     = var.credential_id
  security_group_id = var.security_group_id
  endpoint_id       = var.endpoint_id

  zone_mappings = [{
    zone_id   = "us-east-1a"
    subnet_id = var.subnet_id
  }]
}

variable "credential_id" {
  type = number
}

variable "security_group_id" {
  type = string
}

variable "endpoint_id" {
  type    = string
  default = null
}

variable "subnet_id" {
  type = string
}
