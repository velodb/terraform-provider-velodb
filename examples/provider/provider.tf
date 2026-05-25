terraform {
  required_providers {
    velodb = {
      source  = "velodb/velodb"
      version = "~> 1.1"
    }
  }
}

provider "velodb" {
  host    = var.velodb_host
  api_key = var.velodb_api_key
}

variable "velodb_host" {
  type        = string
  description = "VeloDB Cloud Management API host, without https://."
  default     = "api.velodb.cloud"
}

variable "velodb_api_key" {
  type        = string
  description = "VeloDB Cloud API key."
  sensitive   = true
}
