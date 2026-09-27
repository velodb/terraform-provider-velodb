# List BYOC credential configurations to discover the credential_id needed to
# import a velodb_byoc_credential resource (import ID: aws/<credential_id>).
data "velodb_byoc_credentials" "all" {
  cloud_provider = "aws"
  region         = "us-east-1"
}

# Map of name -> id, handy for locating the ID to import.
output "credential_ids" {
  value = { for c in data.velodb_byoc_credentials.all.credentials : c.name => c.id }
}
