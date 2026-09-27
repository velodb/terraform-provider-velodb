# List BYOC network configurations to discover the network_config_id needed to
# import a velodb_byoc_network resource. The import ID also needs a credential_id
# (aws/<network_config_id>/<credential_id>); the network config does not report
# it, so pair the network_config_id with the credential you manage -- look it up
# with the velodb_byoc_credentials data source if needed.
data "velodb_byoc_network_configs" "all" {
  cloud_provider = "aws"
  region         = "us-east-1"
}

output "network_config_ids" {
  value = { for n in data.velodb_byoc_network_configs.all.network_configs : n.name => n.id }
}
