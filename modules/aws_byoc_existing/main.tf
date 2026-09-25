terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    velodb = {
      source  = "velodb/velodb"
      version = ">= 1.1.8"
    }
  }
}

data "aws_vpc" "selected" {
  id = var.vpc_id
}

data "aws_s3_bucket" "selected" {
  bucket = var.bucket_name
}

data "aws_security_group" "selected" {
  id = var.security_group_id
}

data "aws_vpc_endpoint" "selected" {
  id = var.endpoint_id
}

data "aws_iam_instance_profile" "data_access" {
  name = element(reverse(split("/", var.data_credential_arn)), 0)
}

data "aws_iam_role" "deployment" {
  name = element(reverse(split("/", var.deployment_credential_arn)), 0)
}

data "aws_subnet" "selected" {
  for_each = var.subnet_ids_by_zone

  id = each.value
}

data "aws_route_table" "private" {
  for_each = var.subnet_ids_by_zone

  subnet_id = each.value
}

data "aws_route" "private_default" {
  for_each = data.aws_route_table.private

  route_table_id         = each.value.id
  destination_cidr_block = "0.0.0.0/0"
}

data "aws_nat_gateway" "private" {
  for_each = data.aws_route.private_default

  id = each.value.nat_gateway_id
}

data "velodb_byoc_prerequisites" "aws" {
  cloud_provider = "aws"
  region         = var.region
}

locals {
  zones           = sort(keys(var.subnet_ids_by_zone))
  zone            = local.zones[0]
  supported_zones = toset([for zone in data.velodb_byoc_prerequisites.aws.zones : zone.zone])
  tags            = merge({ managed-by = "terraform" }, var.tags)
}

resource "terraform_data" "name_prefix_guard" {
  input = var.name_prefix

  lifecycle {
    ignore_changes = [input]

    postcondition {
      condition     = self.output == var.name_prefix
      error_message = "name_prefix cannot be changed after creation because it identifies VeloDB resources. Restore the original value or destroy and recreate the deployment."
    }
  }
}

resource "velodb_byoc_credential" "this" {
  cloud_provider            = "aws"
  name                      = "${var.name_prefix}-credential"
  region                    = var.region
  bucket_name               = var.bucket_name
  data_credential_arn       = var.data_credential_arn
  deployment_credential_arn = var.deployment_credential_arn

  lifecycle {
    precondition {
      condition = alltrue([for zone in local.zones : contains(local.supported_zones, zone)])
      error_message = format(
        "VeloDB creation blocked: availability zone(s) [%s] are not supported in %s. Choose zones from the supported set: [%s].",
        join(", ", [for zone in local.zones : zone if !contains(local.supported_zones, zone)]),
        var.region,
        join(", ", sort(tolist(local.supported_zones))),
      )
    }
    precondition {
      condition     = data.aws_s3_bucket.selected.bucket_region == var.region
      error_message = "VeloDB creation blocked: the existing S3 bucket must be in the selected AWS region."
    }
    precondition {
      condition     = data.aws_vpc.selected.enable_dns_support && data.aws_vpc.selected.enable_dns_hostnames
      error_message = "VeloDB creation blocked: the existing VPC must have DNS support and DNS hostnames enabled."
    }
    precondition {
      condition     = data.aws_security_group.selected.vpc_id == var.vpc_id
      error_message = "VeloDB creation blocked: the existing security group must belong to the selected VPC."
    }
    precondition {
      condition     = data.aws_vpc_endpoint.selected.vpc_id == var.vpc_id && data.aws_vpc_endpoint.selected.state == "available"
      error_message = "VeloDB creation blocked: the existing VPC endpoint must belong to the selected VPC and be available."
    }
    precondition {
      condition     = data.aws_vpc_endpoint.selected.service_name == data.velodb_byoc_prerequisites.aws.endpoint_service_name
      error_message = "VeloDB creation blocked: the existing VPC endpoint does not target the VeloDB service for this region."
    }
    precondition {
      condition     = data.aws_iam_instance_profile.data_access.arn == var.data_credential_arn
      error_message = "VeloDB creation blocked: data_credential_arn must identify an existing IAM instance profile in the authenticated AWS account."
    }
    precondition {
      condition     = data.aws_iam_role.deployment.arn == var.deployment_credential_arn
      error_message = "VeloDB creation blocked: deployment_credential_arn must identify an existing IAM role in the authenticated AWS account."
    }
    precondition {
      condition     = alltrue([for zone, subnet in data.aws_subnet.selected : subnet.vpc_id == var.vpc_id && subnet.availability_zone == zone && !subnet.map_public_ip_on_launch])
      error_message = "VeloDB creation blocked: every subnet must be private, belong to the selected VPC, and match its availability-zone key."
    }
    precondition {
      condition     = alltrue([for gateway in data.aws_nat_gateway.private : gateway.state == "available"])
      error_message = "VeloDB creation blocked: every private subnet must have an active default route through an available NAT gateway."
    }
  }
}

resource "velodb_byoc_network" "this" {
  cloud_provider    = "aws"
  name              = "${var.name_prefix}-network"
  credential_id     = velodb_byoc_credential.this.id
  security_group_id = var.security_group_id
  endpoint_id       = var.endpoint_id

  zone_mappings = [for zone in local.zones : {
    zone_id   = zone
    subnet_id = var.subnet_ids_by_zone[zone]
  }]
}

resource "velodb_warehouse" "this" {
  name                 = "${var.name_prefix}-warehouse"
  deployment_mode      = "BYOC"
  cloud_provider       = "aws"
  region               = var.region
  setup_mode           = "advanced"
  credential_id        = velodb_byoc_credential.this.id
  network_config_id    = velodb_byoc_network.this.id
  admin_password       = var.admin_password
  initial_core_version = var.initial_core_version
  tags                 = local.tags

  dynamic "public_access_policy" {
    for_each = var.public_access_policy == null ? [] : [var.public_access_policy]
    content {
      policy = public_access_policy.value.policy

      # rules is a nested attribute (SetNestedAttribute), so it is assigned a
      # list, not written as repeated blocks. Only meaningful for ALLOWLIST_ONLY.
      rules = public_access_policy.value.policy == "ALLOWLIST_ONLY" ? [
        for r in public_access_policy.value.rules : {
          cidr        = r.cidr
          description = r.description
        }
      ] : null
    }
  }

  initial_cluster {
    zone         = local.zone
    compute_vcpu = var.compute_vcpu
    cache_gb     = var.cache_gb

    auto_pause {
      enabled              = var.auto_pause.enabled
      idle_timeout_minutes = var.auto_pause.idle_timeout_minutes
    }
  }

  timeouts {
    create = "45m"
    delete = "30m"
  }
}

resource "velodb_cluster" "additional" {
  for_each = var.additional_clusters

  warehouse_id = velodb_warehouse.this.id
  name         = each.value.name
  cluster_type = "COMPUTE"
  zone         = coalesce(each.value.zone, local.zone)
  compute_vcpu = each.value.compute_vcpu
  cache_gb     = each.value.cache_gb

  auto_pause {
    enabled              = each.value.auto_pause.enabled
    idle_timeout_minutes = each.value.auto_pause.idle_timeout_minutes
  }

  lifecycle {
    precondition {
      condition     = contains(local.zones, coalesce(each.value.zone, local.zone))
      error_message = "Additional cluster ${each.key} must use one of the zones configured for this warehouse."
    }
  }
}
