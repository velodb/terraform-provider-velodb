# Initial version and access policy at warehouse creation.
#
# - `version` pins the engine version at creation (major.minor only, e.g.
#   26.1; three-part versions are rejected). The API selects the newest
#   matching build. Create-only: use `core_version_id` to upgrade an existing
#   warehouse.
# - `access_policy` sets the initial public access policy. BYOC-only and
#   create-only: manage it afterward with the
#   `velodb_warehouse_public_access_policy` resource.

# Deny all public access at creation (rules omitted).
resource "velodb_warehouse" "byoc_deny_all" {
  name              = "byoc-deny-all"
  deployment_mode   = "BYOC"
  cloud_provider    = "aws"
  region            = "us-east-1"
  setup_mode        = "advanced"
  credential_id     = velodb_byoc_credential.aws.id
  network_config_id = velodb_byoc_network.aws.id
  admin_password    = var.admin_password

  version = "26.1"

  access_policy {
    policy = "DENY_ALL"
  }

  initial_cluster {
    zone         = "us-east-1a"
    compute_vcpu = 4
    cache_gb     = 100
  }
}

# Allow only specific CIDR ranges at creation.
resource "velodb_warehouse" "byoc_allowlist" {
  name              = "byoc-allowlist"
  deployment_mode   = "BYOC"
  cloud_provider    = "aws"
  region            = "us-east-1"
  setup_mode        = "advanced"
  credential_id     = velodb_byoc_credential.aws.id
  network_config_id = velodb_byoc_network.aws.id
  admin_password    = var.admin_password

  version = "26.1"

  access_policy {
    policy = "ALLOWLIST_ONLY"

    rules {
      cidr        = "203.0.113.0/24"
      description = "office"
    }

    rules {
      cidr        = "198.51.100.10/32"
      description = "bastion"
    }
  }

  initial_cluster {
    zone         = "us-east-1a"
    compute_vcpu = 4
    cache_gb     = 100
  }
}
