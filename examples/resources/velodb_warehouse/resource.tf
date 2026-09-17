# SaaS warehouse with initial cluster
resource "velodb_warehouse" "saas" {
  name            = "analytics-saas"
  deployment_mode = "SaaS"
  cloud_provider  = "aliyun"
  region          = "cn-beijing"

  admin_password         = var.admin_password
  admin_password_version = 1

  initial_cluster {
    zone         = "cn-beijing-k"
    compute_vcpu = 4
    cache_gb     = 1000
    auto_pause {
      enabled              = false
      idle_timeout_minutes = 30
    }
  }

  timeouts {
    create = "30m"
  }
}

# AWS BYOC warehouse using registered custom infrastructure
resource "velodb_warehouse" "byoc" {
  name              = "analytics-byoc"
  deployment_mode   = "BYOC"
  cloud_provider    = "aws"
  region            = "us-east-1"
  setup_mode        = "advanced"
  credential_id     = velodb_byoc_credential.aws.id
  network_config_id = velodb_byoc_network.aws.id
  admin_password    = var.admin_password

  initial_cluster {
    zone         = "us-east-1a"
    compute_vcpu = 4
    cache_gb     = 100
  }
}

variable "admin_password" {
  type      = string
  sensitive = true
}
