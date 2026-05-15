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

# Existing BYOC warehouse imported into Terraform state
import {
  to = velodb_warehouse.byoc
  id = "AWVA7PYB"
}

resource "velodb_warehouse" "byoc" {
  name            = "test_cli"
  deployment_mode = "BYOC"
  cloud_provider  = "aws"
  region          = "us-east-1"
}

# Output BYOC setup shell command when returned by the API
output "byoc_shell_command" {
  value = velodb_warehouse.byoc.byoc_setup[0].shell_command
}

variable "admin_password" {
  type      = string
  sensitive = true
}
