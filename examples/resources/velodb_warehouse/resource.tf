# SaaS warehouse with initial cluster
resource "velodb_warehouse" "saas" {
  name            = "analytics-saas"
  deployment_mode = "SaaS"
  cloud_provider  = "aws"
  region          = "us-east-1"

  admin_password         = var.admin_password
  admin_password_version = 1

  initial_cluster {
    zone         = "us-east-1a"
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

# AWS BYOC warehouse using registered custom infrastructure.
# Pins the initial engine version and configures the initial public access
# policy at creation. Both are create-only: use core_version_id to upgrade
# later, and velodb_warehouse_public_access_policy to change access afterward.
resource "velodb_warehouse" "byoc" {
  name              = "analytics-byoc"
  deployment_mode   = "BYOC"
  cloud_provider    = "aws"
  region            = "us-east-1"
  setup_mode        = "advanced"
  credential_id     = velodb_byoc_credential.aws.id
  network_config_id = velodb_byoc_network.aws.id
  admin_password    = var.admin_password

  # Provision a specific engine version (major.minor only, e.g. 26.1).
  version = "26.1"

  # Initial public access policy (BYOC only).
  access_policy {
    policy = "ALLOWLIST_ONLY"

    rules {
      cidr        = "203.0.113.0/24"
      description = "office"
    }
  }

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
