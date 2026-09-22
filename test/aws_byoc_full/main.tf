terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    velodb = {
      source  = "velodb/velodb"
      version = ">= 1.1.7"
    }
  }
}

provider "aws" {
  region = var.region
}

provider "velodb" {}

variable "region" {
  type    = string
  default = "us-east-1"
}

variable "bucket_name" {
  description = "Globally unique disposable S3 bucket name."
  type        = string
}

variable "name_prefix" {
  description = "Prefix for disposable AWS and VeloDB resources."
  type        = string
  default     = "tf-byoc-live"

  validation {
    condition     = length(var.name_prefix) <= 20 && can(regex("^[A-Za-z0-9_-]+$", var.name_prefix))
    error_message = "name_prefix must be at most 20 letters, numbers, underscores, or hyphens."
  }
}

variable "admin_password" {
  description = "Warehouse administrator password. Set with TF_VAR_admin_password."
  type        = string
  sensitive   = true
}

variable "create_velodb_resources" {
  description = "Enable only after the AWS-only phase succeeds and IAM has propagated."
  type        = bool
  default     = false
}

variable "create_second_cluster" {
  description = "Create a second cluster after the warehouse phase succeeds."
  type        = bool
  default     = false

  validation {
    condition     = !var.create_second_cluster || var.create_velodb_resources
    error_message = "create_second_cluster requires create_velodb_resources=true."
  }
}

variable "vpc_id" {
  description = "Existing test VPC. Terraform reads but never owns or destroys it."
  type        = string
}

variable "subnet_ids_by_zone" {
  description = "Existing private subnets with working NAT routes, keyed by availability zone."
  type        = map(string)

  validation {
    condition     = length(var.subnet_ids_by_zone) == 3
    error_message = "Exactly three private subnets are required."
  }
}

data "aws_caller_identity" "current" {}

data "aws_vpc" "selected" {
  id = var.vpc_id
}

data "aws_subnet" "selected" {
  for_each = var.subnet_ids_by_zone
  id       = each.value
}

data "velodb_byoc_prerequisites" "aws" {
  cloud_provider = "aws"
  region         = var.region
}

locals {
  zones                   = sort(keys(var.subnet_ids_by_zone))
  zone                    = local.zones[0]
  supported_zones         = toset([for zone in data.velodb_byoc_prerequisites.aws.zones : zone.zone])
  data_access_role_name   = "${var.name_prefix}-data-access"
  deployment_role_name    = "${var.name_prefix}-deployment"
  data_access_role_arn    = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/${local.data_access_role_name}"
  data_access_profile_arn = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:instance-profile/${local.data_access_role_name}"
  tags                    = { managed-by = "terraform", purpose = "velodb-provider-live-test" }
}

###############################################################################
# Provider-generated IAM policies
###############################################################################

data "velodb_aws_data_access_assume_role_policy" "data_access" {
  role_arn = local.data_access_role_arn
}

data "velodb_aws_data_access_policy" "data_access" {
  bucket_name = var.bucket_name
  role_arn    = local.data_access_role_arn
}

data "velodb_aws_assume_role_policy" "deployment" {
  principal_arn = data.velodb_byoc_prerequisites.aws.deployment_assumer_role_arn
  external_id   = data.velodb_byoc_prerequisites.aws.external_id
}

data "velodb_aws_crossaccount_policy" "deployment" {
  bucket_name         = var.bucket_name
  data_credential_arn = local.data_access_profile_arn
}

###############################################################################
# AWS storage and IAM
###############################################################################

resource "aws_s3_bucket" "data" {
  bucket        = var.bucket_name
  force_destroy = true
  tags          = local.tags
}

resource "aws_s3_bucket_public_access_block" "data" {
  bucket = aws_s3_bucket.data.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "data" {
  bucket = aws_s3_bucket.data.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_iam_role" "data_access" {
  name               = local.data_access_role_name
  assume_role_policy = data.velodb_aws_data_access_assume_role_policy.data_access.json
  tags               = local.tags
}

resource "aws_iam_policy" "data_access" {
  name   = "${local.data_access_role_name}-policy"
  policy = data.velodb_aws_data_access_policy.data_access.json
  tags   = local.tags
}

resource "aws_iam_role_policy_attachment" "data_access" {
  role       = aws_iam_role.data_access.name
  policy_arn = aws_iam_policy.data_access.arn
}

resource "aws_iam_instance_profile" "data_access" {
  name = local.data_access_role_name
  role = aws_iam_role.data_access.name
  tags = local.tags
}

resource "aws_iam_role" "deployment" {
  name               = local.deployment_role_name
  assume_role_policy = data.velodb_aws_assume_role_policy.deployment.json
  tags               = local.tags
}

resource "aws_iam_policy" "deployment" {
  name   = "${local.deployment_role_name}-policy"
  policy = data.velodb_aws_crossaccount_policy.deployment.json
  tags   = local.tags
}

resource "aws_iam_role_policy_attachment" "deployment" {
  role       = aws_iam_role.deployment.name
  policy_arn = aws_iam_policy.deployment.arn
}

###############################################################################
# Existing three-zone test network and fresh security resources
###############################################################################

resource "aws_security_group" "warehouse" {
  name        = "${var.name_prefix}-warehouse"
  description = "VeloDB live-test warehouse instances"
  vpc_id      = data.aws_vpc.selected.id
  tags        = local.tags
}

resource "aws_vpc_security_group_ingress_rule" "warehouse_self" {
  security_group_id            = aws_security_group.warehouse.id
  ip_protocol                  = "tcp"
  from_port                    = 0
  to_port                      = 65535
  referenced_security_group_id = aws_security_group.warehouse.id
}

resource "aws_vpc_security_group_egress_rule" "warehouse_all" {
  security_group_id = aws_security_group.warehouse.id
  ip_protocol       = "-1"
  cidr_ipv4         = "0.0.0.0/0"
}

resource "aws_security_group" "endpoint" {
  name        = "${var.name_prefix}-endpoint"
  description = "VeloDB live-test PrivateLink endpoint"
  vpc_id      = data.aws_vpc.selected.id
  tags        = local.tags
}

resource "aws_vpc_security_group_ingress_rule" "endpoint_https" {
  security_group_id = aws_security_group.endpoint.id
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  cidr_ipv4         = data.aws_vpc.selected.cidr_block
}

resource "aws_vpc_security_group_egress_rule" "endpoint_all" {
  security_group_id = aws_security_group.endpoint.id
  ip_protocol       = "-1"
  cidr_ipv4         = "0.0.0.0/0"
}

resource "aws_vpc_endpoint" "velodb" {
  vpc_id              = data.aws_vpc.selected.id
  service_name        = data.velodb_byoc_prerequisites.aws.endpoint_service_name
  vpc_endpoint_type   = "Interface"
  subnet_ids          = values(var.subnet_ids_by_zone)
  security_group_ids  = [aws_security_group.endpoint.id]
  private_dns_enabled = false
  tags                = merge(local.tags, { Name = "${var.name_prefix}-velodb" })

  depends_on = [
    aws_vpc_security_group_ingress_rule.endpoint_https,
    aws_vpc_security_group_egress_rule.endpoint_all,
  ]
}

###############################################################################
# VeloDB registrations and warehouse (phase two)
###############################################################################

resource "velodb_byoc_credential" "test" {
  count = var.create_velodb_resources ? 1 : 0

  cloud_provider            = "aws"
  name                      = "${var.name_prefix}-credential"
  region                    = var.region
  bucket_name               = aws_s3_bucket.data.bucket
  data_credential_arn       = aws_iam_instance_profile.data_access.arn
  deployment_credential_arn = aws_iam_role.deployment.arn

  depends_on = [
    aws_iam_role_policy_attachment.data_access,
    aws_iam_role_policy_attachment.deployment,
    aws_s3_bucket_public_access_block.data,
    aws_s3_bucket_ownership_controls.data,
  ]
}

resource "velodb_byoc_network" "test" {
  count = var.create_velodb_resources ? 1 : 0

  cloud_provider    = "aws"
  name              = "${var.name_prefix}-network"
  credential_id     = velodb_byoc_credential.test[0].id
  security_group_id = aws_security_group.warehouse.id
  endpoint_id       = aws_vpc_endpoint.velodb.id

  zone_mappings = [for zone in local.zones : {
    zone_id   = zone
    subnet_id = var.subnet_ids_by_zone[zone]
  }]

  depends_on = [
    aws_vpc_security_group_ingress_rule.warehouse_self,
    aws_vpc_security_group_egress_rule.warehouse_all,
  ]
}

resource "velodb_warehouse" "test" {
  count = var.create_velodb_resources ? 1 : 0

  name              = "${var.name_prefix}-warehouse"
  deployment_mode   = "BYOC"
  cloud_provider    = "aws"
  region            = var.region
  setup_mode        = "advanced"
  credential_id     = velodb_byoc_credential.test[0].id
  network_config_id = velodb_byoc_network.test[0].id
  admin_password    = var.admin_password

  initial_cluster {
    zone         = local.zone
    compute_vcpu = 4
    cache_gb     = 100
    auto_pause {
      enabled = false
    }
  }

  timeouts {
    create = "45m"
    delete = "30m"
  }
}

resource "velodb_cluster" "second" {
  count = var.create_second_cluster ? 1 : 0

  warehouse_id = velodb_warehouse.test[0].id
  name         = "${replace(var.name_prefix, "-", "_")}_second"
  cluster_type = "COMPUTE"
  zone         = local.zone
  compute_vcpu = 4
  cache_gb     = 100

  auto_pause {
    enabled = false
  }
}

check "policy_documents" {
  assert {
    condition     = length(jsondecode(data.velodb_aws_crossaccount_policy.deployment.json).Statement) == 17
    error_message = "Deployment policy must contain 17 statements."
  }
  assert {
    condition     = length(jsondecode(data.velodb_aws_data_access_policy.data_access.json).Statement) == 3
    error_message = "Data-access policy must contain three statements without TDE."
  }
}

check "selected_network" {
  assert {
    condition = alltrue([
      for zone, subnet in data.aws_subnet.selected :
      subnet.vpc_id == data.aws_vpc.selected.id &&
      subnet.availability_zone == zone &&
      !subnet.map_public_ip_on_launch
    ])
    error_message = "Every selected subnet must be private, in the selected VPC, and keyed by its actual zone."
  }
  assert {
    condition     = alltrue([for zone in local.zones : contains(local.supported_zones, zone)])
    error_message = "Every selected subnet zone must be supported by the VeloDB BYOC API."
  }
}

output "test_status" {
  value = {
    aws_account_id     = data.aws_caller_identity.current.account_id
    zones              = local.zones
    bucket             = aws_s3_bucket.data.bucket
    vpc_id             = data.aws_vpc.selected.id
    private_subnet_ids = var.subnet_ids_by_zone
    endpoint_id        = aws_vpc_endpoint.velodb.id
    credential_id      = try(velodb_byoc_credential.test[0].id, null)
    network_config_id  = try(velodb_byoc_network.test[0].id, null)
    warehouse_id       = try(velodb_warehouse.test[0].id, null)
    warehouse_status   = try(velodb_warehouse.test[0].status, null)
    initial_cluster_id = try(velodb_warehouse.test[0].initial_cluster_id, null)
    second_cluster_id  = try(velodb_cluster.second[0].id, null)
  }
}
