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

data "aws_caller_identity" "current" {}

data "velodb_byoc_prerequisites" "aws" {
  cloud_provider = "aws"
  region         = var.region
}

locals {
  zone                    = var.zones[0]
  supported_zones         = toset([for zone in data.velodb_byoc_prerequisites.aws.zones : zone.zone])
  private_subnet_ids      = { for zone, subnet in aws_subnet.private : zone => subnet.id }
  data_access_role_name   = "${var.name_prefix}-data-access"
  deployment_role_name    = "${var.name_prefix}-deployment"
  data_access_role_arn    = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/${local.data_access_role_name}"
  data_access_profile_arn = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:instance-profile/${local.data_access_role_name}"
  tags                    = merge({ managed-by = "terraform" }, var.tags)
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
  force_destroy = var.bucket_force_destroy
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

resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.this.id
  availability_zone       = local.zone
  cidr_block              = cidrsubnet(var.vpc_cidr, 8, 0)
  map_public_ip_on_launch = true
  tags                    = merge(local.tags, { Name = "${var.name_prefix}-public-${local.zone}" })
}

resource "aws_subnet" "private" {
  for_each = toset(var.zones)

  vpc_id                  = aws_vpc.this.id
  availability_zone       = each.key
  cidr_block              = cidrsubnet(var.vpc_cidr, 4, index(var.zones, each.key) + 1)
  map_public_ip_on_launch = false
  tags                    = merge(local.tags, { Name = "${var.name_prefix}-private-${each.key}" })
}

resource "aws_eip" "nat" {
  domain = "vpc"
  tags   = merge(local.tags, { Name = "${var.name_prefix}-nat" })
}

# ponytail: one NAT keeps the basic module affordable; add a high-availability module when users require one NAT per AZ.
resource "aws_nat_gateway" "this" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public.id
  tags          = merge(local.tags, { Name = "${var.name_prefix}-nat" })

  depends_on = [aws_internet_gateway.this]
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id
  tags   = merge(local.tags, { Name = "${var.name_prefix}-public" })
}

resource "aws_route" "public_internet" {
  route_table_id         = aws_route_table.public.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.this.id
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
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

  depends_on = [
    aws_iam_role_policy_attachment.data_access,
    aws_iam_role_policy_attachment.deployment,
    aws_s3_bucket_public_access_block.data,
    aws_s3_bucket_ownership_controls.data,
    aws_route_table_association.public,
    aws_route_table_association.private,
    aws_vpc_security_group_ingress_rule.endpoint_https,
    aws_vpc_security_group_egress_rule.endpoint_all,
    aws_vpc_security_group_ingress_rule.warehouse_self,
    aws_vpc_security_group_egress_rule.warehouse_all,
  ]

  lifecycle {
    precondition {
      condition     = alltrue([for zone in var.zones : contains(local.supported_zones, zone)])
      error_message = "VeloDB creation blocked: one or more selected availability zones are unsupported. Choose three zones returned by velodb_byoc_prerequisites."
    }
    precondition {
      condition     = aws_route.public_internet.state == "active" && aws_route.private_nat.state == "active"
      error_message = "VeloDB creation blocked: AWS outbound routing is not active. Verify the internet gateway, NAT gateway, public route, and private default route before retrying."
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
  name              = "${var.name_prefix}-warehouse"
  deployment_mode   = "BYOC"
  cloud_provider    = "aws"
  region            = var.region
  setup_mode        = "advanced"
  credential_id     = velodb_byoc_credential.this.id
  network_config_id = velodb_byoc_network.this.id
  admin_password    = var.admin_password

  initial_cluster {
    zone         = local.zone
    compute_vcpu = var.compute_vcpu
    cache_gb     = var.cache_gb
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

  warehouse_id = velodb_warehouse.this.id
  name         = "${replace(var.name_prefix, "-", "_")}_second"
  cluster_type = "COMPUTE"
  zone         = local.zone
  compute_vcpu = var.compute_vcpu
  cache_gb     = var.cache_gb

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
