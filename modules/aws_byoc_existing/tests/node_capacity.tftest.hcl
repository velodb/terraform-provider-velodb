mock_provider "aws" {
  mock_data "aws_caller_identity" { defaults = { account_id = "111122223333" } }
  mock_data "aws_vpc" { defaults = { enable_dns_support = true, enable_dns_hostnames = true } }
  mock_data "aws_s3_bucket" { defaults = { bucket_region = "us-east-1" } }
  mock_data "aws_security_group" { defaults = { vpc_id = "vpc-test" } }
  mock_data "aws_vpc_endpoint" { defaults = { vpc_id = "vpc-test", state = "available", service_name = "com.amazonaws.vpce.us-east-1.vpce-svc-test" } }
  mock_data "aws_iam_instance_profile" { defaults = { arn = "arn:aws:iam::111122223333:instance-profile/data" } }
  mock_data "aws_iam_role" { defaults = { arn = "arn:aws:iam::111122223333:role/deploy" } }
  mock_data "aws_subnet" { defaults = { vpc_id = "vpc-test", availability_zone = "us-east-1a", map_public_ip_on_launch = false } }
  mock_data "aws_nat_gateway" { defaults = { state = "available" } }
}
mock_provider "velodb" {
  mock_data "velodb_aws_data_access_assume_role_policy" { defaults = { json = "{}" } }
  mock_data "velodb_aws_assume_role_policy" { defaults = { json = "{}" } }

  mock_data "velodb_aws_crossaccount_policy" { defaults = { json = "{\"Statement\":[{},{},{},{},{},{},{},{},{},{},{},{},{},{},{},{},{}]}" } }
  mock_data "velodb_aws_data_access_policy" { defaults = { json = "{\"Statement\":[{},{},{}]}" } }
}
override_data {
  target = data.velodb_byoc_prerequisites.aws
  values = {
    deployment_assumer_role_arn = "arn:aws:iam::111122223333:role/deploy"
    endpoint_service_name       = "com.amazonaws.vpce.us-east-1.vpce-svc-test"
    external_id                 = "capacity-test"
  }
}
variables {
  region                    = "us-east-1"
  bucket_name               = "capacity-module-test"
  name_prefix               = "capacity-test"
  admin_password            = "TestPass@123"
  vpc_id                    = "vpc-test"
  subnet_ids_by_zone        = { "us-east-1a" = "subnet-test" }
  security_group_id         = "sg-test"
  endpoint_id               = "vpce-test"
  data_credential_arn       = "arn:aws:iam::111122223333:instance-profile/data"
  deployment_credential_arn = "arn:aws:iam::111122223333:role/deploy"
}
run "legacy_defaults" {
  command = plan
  plan_options { target = [terraform_data.warehouse_infrastructure_guard] }
  assert {
    condition     = local.initial_compute_vcpu == 4 && local.initial_cache_gb == 100 && !local.node_capacity
    error_message = "Existing module defaults must remain unchanged."
  }
}
run "node_inputs_suppress_legacy_defaults" {
  command = plan
  plan_options { target = [terraform_data.warehouse_infrastructure_guard] }
  variables {
    node_vcpu           = 8
    node_count          = 3
    node_cache          = 200
    additional_clusters = { worker = { name = "worker", node_vcpu = 8, node_count = 2, node_cache = 200 } }
  }
  assert {
    condition     = local.initial_compute_vcpu == null && local.initial_cache_gb == null && local.node_capacity
    error_message = "Node requests must omit legacy capacity fields."
  }
}
run "additional_cluster_missing_node_cache" {
  command = plan
  variables { additional_clusters = { worker = { name = "worker", node_vcpu = 8, node_count = 2 } } }
  # Mock providers leave the computed zone collection empty. Require the
  # capacity diagnostic as well as the unrelated credential-zone diagnostic.
  expect_failures = [var.additional_clusters, velodb_byoc_credential.this]
}
run "additional_cluster_mixed_capacity" {
  command = plan
  variables { additional_clusters = { worker = { name = "worker", compute_vcpu = 16, cache_gb = 400, node_vcpu = 8, node_count = 2, node_cache = 200 } } }
  # Mock providers leave the computed zone collection empty. Require the
  # capacity diagnostic as well as the unrelated credential-zone diagnostic.
  expect_failures = [var.additional_clusters, velodb_byoc_credential.this]
}

run "local_nodes" {
  command = plan
  plan_options { target = [terraform_data.warehouse_infrastructure_guard] }
  variables {
    cache_type          = 1
    node_vcpu           = 4
    node_count          = 2
    additional_clusters = { worker = { name = "worker", cache_type = 1, node_vcpu = 8, node_count = 2 } }
  }
  assert {
    condition     = local.node_capacity && local.initial_compute_vcpu == null && local.initial_cache_gb == null && var.node_cache == null
    error_message = "Local cache must omit cache size and legacy inputs."
  }
}
run "local_cache_configuration_rejected" {
  command = plan
  variables { additional_clusters = { worker = { name = "worker", cache_type = 1, node_vcpu = 8, node_count = 2, node_cache = 474 } } }
  # Mock providers leave the computed zone collection empty. Require the
  # capacity diagnostic as well as the unrelated credential-zone diagnostic.
  expect_failures = [var.additional_clusters, velodb_byoc_credential.this]
}
run "invalid_cache_type" {
  command = plan
  plan_options { target = [terraform_data.warehouse_infrastructure_guard] }
  variables { cache_type = 2 }
  expect_failures = [var.cache_type]
}
