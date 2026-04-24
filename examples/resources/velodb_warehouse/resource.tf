# SaaS warehouse with initial cluster
resource "velodb_warehouse" "saas" {
  name            = "analytics-saas"
  deployment_mode = "SAAS"
  cloud_provider  = "aliyun"
  region          = "cn-beijing"

  admin_password         = var.admin_password
  admin_password_version = 1

  advanced_settings = jsonencode({ enableTde = 0 })

  initial_cluster {
    name         = "default"
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

# BYOC warehouse with Template mode
resource "velodb_warehouse" "byoc" {
  name            = "production-byoc"
  deployment_mode = "BYOC"
  cloud_provider  = "aliyun"
  region          = "cn-beijing"
  setup_mode     = "guided"
  vpc_mode        = "existing"
  vpc_id          = "vpc-xxxxxx"

  admin_password         = var.admin_password
  admin_password_version = 1

  initial_cluster {
    name           = "default-compute"
    zone           = "cn-beijing-k"
    compute_vcpu   = 8
    cache_gb       = 400
    billing_method = "monthly"
    auto_pause {
      enabled              = true
      idle_timeout_minutes = 30
    }
  }

  timeouts {
    create = "45m"
  }
}

# Output BYOC setup shell command
output "byoc_shell_command" {
  value = velodb_warehouse.byoc.byoc_setup[0].shell_command
}

variable "admin_password" {
  type      = string
  sensitive = true
}
