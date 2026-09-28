# Initial version and access policy at warehouse creation.
#
# - `core_version` accepts two or three parts at creation. Two parts let the
#   backend select the latest patch. Change to a three-part target to upgrade
#   the same warehouse in place, using the same core_version field.
# - `public_access_policy` sets the initial BYOC public access policy and
#   updates it in place afterward. Remote policy changes are detected on refresh.

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

  core_version = "26.1"

  public_access_policy {
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

  core_version = "26.1"

  public_access_policy {
    policy = "ALLOWLIST_ONLY"

    rules = [
      {
        cidr        = "203.0.113.0/24"
        description = "office"
      },
      {
        cidr        = "198.51.100.10/32"
        description = "bastion"
      },
    ]
  }

  initial_cluster {
    zone         = "us-east-1a"
    compute_vcpu = 4
    cache_gb     = 100
  }
}
