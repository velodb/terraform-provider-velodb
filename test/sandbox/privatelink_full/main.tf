terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    velodb = {
      source = "velodb/velodb"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

provider "velodb" {
  host    = "sandbox-api.velodb.io"
  api_key = var.api_key
}

variable "api_key" {
  type      = string
  sensitive = true
}

variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "warehouse_id" {
  type = string
}

variable "warehouse_endpoint_service_name" {
  type        = string
  description = "Inbound PrivateLink service name returned by the VeloDB warehouse."
}

variable "name_suffix" {
  type    = string
  default = "manual"
}

locals {
  prefix = "tfpl-e2e-${var.name_suffix}"
}

data "aws_vpc_endpoint_service" "velodb_inbound" {
  service_name = var.warehouse_endpoint_service_name
}

locals {
  selected_az = tolist(data.aws_vpc_endpoint_service.velodb_inbound.availability_zones)[0]
}

resource "aws_vpc" "main" {
  cidr_block           = "10.91.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = local.prefix
  }
}

resource "aws_subnet" "main" {
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.91.1.0/24"
  availability_zone = local.selected_az

  tags = {
    Name = "${local.prefix}-subnet"
  }
}

resource "aws_security_group" "endpoint" {
  name        = "${local.prefix}-endpoint"
  description = "Temporary VeloDB PrivateLink e2e security group"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "Allow VPC traffic to endpoint"
    from_port   = 0
    to_port     = 65535
    protocol    = "tcp"
    cidr_blocks = [aws_vpc.main.cidr_block]
  }

  egress {
    description = "Allow all egress"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${local.prefix}-endpoint"
  }
}

resource "aws_vpc_endpoint" "inbound" {
  vpc_id              = aws_vpc.main.id
  service_name        = var.warehouse_endpoint_service_name
  vpc_endpoint_type   = "Interface"
  subnet_ids          = [aws_subnet.main.id]
  security_group_ids  = [aws_security_group.endpoint.id]
  private_dns_enabled = false

  tags = {
    Name = "${local.prefix}-inbound"
  }
}

resource "velodb_warehouse_private_endpoint" "inbound" {
  warehouse_id = var.warehouse_id
  endpoint_id  = aws_vpc_endpoint.inbound.id
  dns_name     = "${local.prefix}.internal"
  description  = "Terraform full inbound PrivateLink e2e"
}

resource "aws_lb" "outbound" {
  name                       = substr(replace("${local.prefix}-nlb", "_", "-"), 0, 32)
  internal                   = true
  load_balancer_type         = "network"
  subnets                    = [aws_subnet.main.id]
  enable_deletion_protection = false

  tags = {
    Name = "${local.prefix}-nlb"
  }
}

resource "aws_vpc_endpoint_service" "outbound" {
  acceptance_required        = false
  network_load_balancer_arns = [aws_lb.outbound.arn]

  tags = {
    Name = "${local.prefix}-service"
  }
}

resource "aws_vpc_endpoint_service_allowed_principal" "outbound_all" {
  vpc_endpoint_service_id = aws_vpc_endpoint_service.outbound.id
  principal_arn           = "*"
}

resource "velodb_private_link_endpoint_service" "outbound" {
  cloud_provider        = "aws"
  region                = var.aws_region
  endpoint_service_id   = aws_vpc_endpoint_service.outbound.id
  endpoint_service_name = aws_vpc_endpoint_service.outbound.service_name
  description           = "Terraform full outbound PrivateLink e2e"

  depends_on = [aws_vpc_endpoint_service_allowed_principal.outbound_all]
}

data "velodb_warehouse_connections" "after_inbound" {
  warehouse_id = var.warehouse_id

  depends_on = [velodb_warehouse_private_endpoint.inbound]
}

output "inbound_endpoint_id" {
  value = aws_vpc_endpoint.inbound.id
}

output "inbound_private_endpoint_count" {
  value = length(data.velodb_warehouse_connections.after_inbound.private_endpoints)
}

output "outbound_endpoint_service_id" {
  value = velodb_private_link_endpoint_service.outbound.id
}

output "outbound_connected" {
  value = velodb_private_link_endpoint_service.outbound.connected
}
