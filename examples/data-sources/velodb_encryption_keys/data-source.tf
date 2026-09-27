# List registered encryption keys to discover the encryption_key_id needed to
# import a velodb_encryption_key resource (import ID: aws/<encryption_key_id>).
data "velodb_encryption_keys" "all" {
  cloud_provider = "aws"
  region         = "us-east-1"
}

output "encryption_key_ids" {
  value = { for k in data.velodb_encryption_keys.all.encryption_keys : k.name => k.id }
}
