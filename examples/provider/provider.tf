terraform {
  required_providers {
    velodb = {
      source  = "velodb/velodb"
      version = "~> 0.1"
    }
  }
}

provider "velodb" {
  host    = var.velodb_host
  api_key = var.velodb_api_key
}

variable "velodb_host" {
  type        = string
  description = "VeloDB API host"
  default     = "api.selectdbcloud.com"
}

variable "velodb_api_key" {
  type        = string
  description = "VeloDB API key"
  sensitive   = true
}
