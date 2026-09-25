terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 6.24.0"
    }
    velodb = {
      source  = "velodb/velodb"
      version = ">= 1.1.8"
    }
  }
}

data "aws_caller_identity" "current" {}

data "velodb_byoc_prerequisites" "aws" {
  cloud_provider = "aws"
  region         = var.region
}

locals {
  zone               = var.zones[0]
  supported_zones    = toset([for zone in data.velodb_byoc_prerequisites.aws.zones : zone.zone])
  private_subnet_ids = { for zone, subnet in aws_subnet.private : zone => subnet.id }

  # Per-AZ /20 subnet CIDRs, keyed on the zone letter so each AZ always maps to
  # the same block regardless of its position in var.zones. Deriving the block
  # from the list index instead would let a newly added zone inherit the CIDR of
  # a zone being removed (e.g. swapping us-east-1b for us-east-1c). The old
  # subnet cannot be deleted while the VeloDB VPC endpoint ENI still occupies it,
  # and the new subnet cannot be created because its CIDR conflicts, deadlocking
  # the replacement. Index 0 stays reserved so the letters map to blocks 1-8.
  # An explicit var.subnet_cidrs entry overrides the derived block for a zone and
  # short-circuits the letter lookup, so non-standard zones can be placed too.
  az_letters = ["a", "b", "c", "d", "e", "f", "g", "h"]
  subnet_cidr_by_zone = { for zone in var.zones : zone => (
    contains(keys(var.subnet_cidrs), zone)
    ? var.subnet_cidrs[zone]
    : cidrsubnet(var.vpc_cidr, 4, index(local.az_letters, trimprefix(zone, var.region)) + 1)
  ) }
  data_access_role_name   = "${var.name_prefix}-data-access"
  deployment_role_name    = "${var.name_prefix}-deployment"
  data_access_role_arn    = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/${local.data_access_role_name}"
  data_access_profile_arn = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:instance-profile/${local.data_access_role_name}"
  tags                    = merge({ managed-by = "terraform" }, var.tags)
}

resource "terraform_data" "name_prefix_guard" {
  input = var.name_prefix

  lifecycle {
    ignore_changes = [input]

    postcondition {
      condition     = self.output == var.name_prefix
      error_message = "name_prefix cannot be changed after creation because it identifies AWS and VeloDB resources. Restore the original value or destroy and recreate the deployment."
    }
  }
}

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

resource "aws_s3_bucket" "data" {
  bucket        = var.bucket_name
  force_destroy = false
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

resource "aws_vpc" "this" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags                 = merge(local.tags, { Name = "${var.name_prefix}-vpc" })
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
  tags   = merge(local.tags, { Name = "${var.name_prefix}-igw" })
}

resource "aws_subnet" "private" {
  for_each = toset(var.zones)

  vpc_id                  = aws_vpc.this.id
  availability_zone       = each.key
  cidr_block              = local.subnet_cidr_by_zone[each.key]
  map_public_ip_on_launch = false
  tags                    = merge(local.tags, { Name = "${var.name_prefix}-private-${each.key}" })

  lifecycle {
    precondition {
      condition     = alltrue([for zone in keys(var.subnet_cidrs) : contains(var.zones, zone)])
      error_message = "Every subnet_cidrs key must be one of the configured zones. Remove or correct overrides for zones not listed in var.zones."
    }
    precondition {
      condition     = length(distinct(values(local.subnet_cidr_by_zone))) == length(var.zones)
      error_message = "Effective subnet CIDRs must be unique across zones. A subnet_cidrs override collides with another zone's CIDR."
    }
  }
}

# A regional NAT gateway (availability_mode = "regional", automatic mode) provides
# multi-AZ high availability by default: it expands and contracts across AZs with the
# workload and keeps zonal affinity, so an AZ outage does not cut egress for the
# surviving AZs. It needs no public subnet and manages its own EIPs. Requires the
# hashicorp/aws provider >= 6.24.0.
resource "aws_nat_gateway" "this" {
  vpc_id            = aws_vpc.this.id
  availability_mode = "regional"
  tags              = merge(local.tags, { Name = "${var.name_prefix}-nat" })

  depends_on = [aws_internet_gateway.this]
}

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.this.id
  tags   = merge(local.tags, { Name = "${var.name_prefix}-private" })
}

resource "aws_route" "private_nat" {
  route_table_id         = aws_route_table.private.id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.this.id
}

resource "aws_route_table_association" "private" {
  for_each = aws_subnet.private

  subnet_id      = each.value.id
  route_table_id = aws_route_table.private.id
}

resource "aws_vpc_endpoint" "s3" {
  vpc_id            = aws_vpc.this.id
  service_name      = "com.amazonaws.${var.region}.s3"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = [aws_route_table.private.id]
  tags              = merge(local.tags, { Name = "${var.name_prefix}-s3" })
}

resource "aws_security_group" "warehouse" {
  name        = "${var.name_prefix}-warehouse"
  description = "VeloDB BYOC warehouse instances"
  vpc_id      = aws_vpc.this.id
  tags        = local.tags
}

resource "aws_vpc_security_group_ingress_rule" "warehouse_self" {
  security_group_id            = aws_security_group.warehouse.id
  ip_protocol                  = "tcp"
  from_port                    = 0
  to_port                      = 65535
  referenced_security_group_id = aws_security_group.warehouse.id
}

resource "aws_vpc_security_group_ingress_rule" "warehouse_client" {
  for_each = toset(var.warehouse_client_cidrs)

  security_group_id = aws_security_group.warehouse.id
  ip_protocol       = "tcp"
  from_port         = 8000
  to_port           = 10000
  cidr_ipv4         = each.value
  description       = "Warehouse query access from client VPC CIDRs"
}

resource "aws_vpc_security_group_egress_rule" "warehouse_all" {
  security_group_id = aws_security_group.warehouse.id
  ip_protocol       = "-1"
  cidr_ipv4         = "0.0.0.0/0"
}

resource "aws_security_group" "endpoint" {
  name        = "${var.name_prefix}-endpoint"
  description = "VeloDB BYOC PrivateLink endpoint"
  vpc_id      = aws_vpc.this.id
  tags        = local.tags
}

resource "aws_vpc_security_group_ingress_rule" "endpoint_https" {
  security_group_id = aws_security_group.endpoint.id
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  cidr_ipv4         = var.vpc_cidr
}

resource "aws_vpc_security_group_egress_rule" "endpoint_all" {
  security_group_id = aws_security_group.endpoint.id
  ip_protocol       = "-1"
  cidr_ipv4         = "0.0.0.0/0"
}

resource "aws_vpc_endpoint" "velodb" {
  vpc_id              = aws_vpc.this.id
  service_name        = data.velodb_byoc_prerequisites.aws.endpoint_service_name
  vpc_endpoint_type   = "Interface"
  subnet_ids          = values(local.private_subnet_ids)
  security_group_ids  = [aws_security_group.endpoint.id]
  private_dns_enabled = false
  tags                = merge(local.tags, { Name = "${var.name_prefix}-velodb" })

  depends_on = [
    aws_vpc_security_group_ingress_rule.endpoint_https,
    aws_vpc_security_group_egress_rule.endpoint_all,
  ]
}

resource "velodb_byoc_credential" "this" {
  cloud_provider            = "aws"
  name                      = "${var.name_prefix}-credential"
  region                    = var.region
  bucket_name               = aws_s3_bucket.data.bucket
  data_credential_arn       = aws_iam_instance_profile.data_access.arn
  deployment_credential_arn = aws_iam_role.deployment.arn

  # The credential is the last VeloDB resource destroyed (the warehouse and
  # network both reference credential_id, so it outlives them). Anchoring every
  # underlying AWS resource the VeloDB backend needs for provisioning and
  # deprovisioning to the credential via depends_on guarantees they are torn down
  # only after the credential -- and therefore after the warehouse has finished
  # deleting -- instead of in parallel with it. The S3 gateway endpoint and the
  # NAT default route are otherwise referenced only from the preconditions below,
  # and precondition graph edges are not a reliable basis for destroy ordering,
  # so they are listed here explicitly.
  depends_on = [
    aws_iam_role_policy_attachment.data_access,
    aws_iam_role_policy_attachment.deployment,
    aws_s3_bucket_public_access_block.data,
    aws_s3_bucket_ownership_controls.data,
    aws_route_table_association.private,
    aws_route.private_nat,
    aws_vpc_endpoint.s3,
    aws_vpc_security_group_ingress_rule.endpoint_https,
    aws_vpc_security_group_egress_rule.endpoint_all,
    aws_vpc_security_group_ingress_rule.warehouse_self,
    aws_vpc_security_group_ingress_rule.warehouse_client,
    aws_vpc_security_group_egress_rule.warehouse_all,
  ]

  lifecycle {
    precondition {
      condition = alltrue([for zone in var.zones : contains(local.supported_zones, zone)])
      error_message = format(
        "VeloDB creation blocked: availability zone(s) [%s] are not supported in %s. Choose one or three zones from the supported set: [%s].",
        join(", ", [for zone in var.zones : zone if !contains(local.supported_zones, zone)]),
        var.region,
        join(", ", sort(tolist(local.supported_zones))),
      )
    }
    precondition {
      condition     = aws_route.private_nat.state == "active"
      error_message = "VeloDB creation blocked: AWS outbound routing is not active. Verify the internet gateway, regional NAT gateway, and private default route before retrying."
    }
    precondition {
      condition     = aws_vpc_endpoint.s3.state == "available" && aws_vpc_endpoint.velodb.state == "available"
      error_message = "VeloDB creation blocked: required AWS VPC endpoints are not available. Check the S3 and VeloDB endpoint states before retrying."
    }
  }
}

resource "velodb_byoc_network" "this" {
  cloud_provider    = "aws"
  name              = "${var.name_prefix}-network"
  credential_id     = velodb_byoc_credential.this.id
  security_group_id = aws_security_group.warehouse.id
  endpoint_id       = aws_vpc_endpoint.velodb.id

  zone_mappings = [for zone in var.zones : {
    zone_id   = zone
    subnet_id = local.private_subnet_ids[zone]
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
      condition     = contains(var.zones, coalesce(each.value.zone, local.zone))
      error_message = "Additional cluster ${each.key} must use one of the zones configured for this warehouse."
    }
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
