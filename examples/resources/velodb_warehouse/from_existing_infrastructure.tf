# Create a BYOC warehouse from EXISTING VeloDB infrastructure.
#
# Use this when the credential, network, and encryption keys already exist in
# your VeloDB organization -- created out of band, in another Terraform state, or
# imported (see examples/resources/velodb_byoc_credential/import.sh). Instead of
# hard-coding the numeric IDs, look them up by name with the discovery data
# sources so the warehouse always binds to the current IDs.

# Discover the existing credential, network config, and encryption keys.
data "velodb_byoc_credentials" "existing" {
  cloud_provider = "aws"
  region         = "us-east-1"
}

data "velodb_byoc_network_configs" "existing" {
  cloud_provider = "aws"
  region         = "us-east-1"
}

data "velodb_encryption_keys" "existing" {
  cloud_provider = "aws"
  region         = "us-east-1"
}

# Select the specific objects by name. one() asserts exactly one match, so a
# typo or a missing object fails the plan with a clear error instead of silently
# picking the wrong ID.
locals {
  credential_id     = one([for c in data.velodb_byoc_credentials.existing.credentials : c.id if c.name == "production-credential"])
  network_config_id = one([for n in data.velodb_byoc_network_configs.existing.network_configs : n.id if n.name == "production-network"])
  tde_key_id        = one([for k in data.velodb_encryption_keys.existing.encryption_keys : k.id if k.name == "production-tde"])
  ebs_key_id        = one([for k in data.velodb_encryption_keys.existing.encryption_keys : k.id if k.name == "production-ebs"])
}

# Create the warehouse against the discovered infrastructure. credential_id,
# network_config_id, and the encryption key IDs are create-only (changing any of
# them replaces the warehouse).
resource "velodb_warehouse" "from_existing" {
  name              = "analytics-existing"
  deployment_mode   = "BYOC"
  cloud_provider    = "aws"
  region            = "us-east-1"
  setup_mode        = "advanced"
  credential_id     = local.credential_id
  network_config_id = local.network_config_id
  admin_password    = var.admin_password

  tde_encryption_key_id = local.tde_key_id
  ebs_encryption_key_id = local.ebs_key_id

  initial_cluster {
    zone         = "us-east-1a"
    compute_vcpu = 4
    cache_gb     = 100
  }
}
