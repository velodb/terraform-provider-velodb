variable "region" {
  description = "AWS region containing the existing BYOC infrastructure."
  type        = string
}

variable "bucket_name" {
  description = "Existing S3 bucket name. This module does not manage or delete it."
  type        = string
}

variable "name_prefix" {
  description = "Prefix for VeloDB resources. Do not change after creation."
  type        = string

  validation {
    condition     = length(var.name_prefix) <= 20 && can(regex("^[A-Za-z0-9_-]+$", var.name_prefix))
    error_message = "name_prefix must be at most 20 letters, numbers, underscores, or hyphens."
  }
}

variable "admin_password" {
  description = "Warehouse administrator password."
  type        = string
  sensitive   = true
}

variable "vpc_id" {
  description = "Existing VPC ID. This module does not manage or delete it."
  type        = string
}

variable "subnet_ids_by_zone" {
  description = "Three existing private subnet IDs keyed by VeloDB-supported availability zone."
  type        = map(string)

  validation {
    condition     = length(var.subnet_ids_by_zone) == 3 && length(distinct(values(var.subnet_ids_by_zone))) == 3
    error_message = "Provide exactly three distinct subnet IDs keyed by availability zone."
  }
}

variable "security_group_id" {
  description = "Existing warehouse security group ID. This module does not manage or delete it."
  type        = string
}

variable "endpoint_id" {
  description = "Existing VeloDB interface VPC endpoint ID. This module does not manage or delete it."
  type        = string
}

variable "data_credential_arn" {
  description = "Existing AWS IAM instance-profile ARN used for warehouse data access."
  type        = string

  validation {
    condition     = can(regex("^arn:[^:]+:iam::[0-9]{12}:instance-profile/.+$", var.data_credential_arn))
    error_message = "data_credential_arn must be an IAM instance-profile ARN."
  }
}

variable "deployment_credential_arn" {
  description = "Existing AWS IAM role ARN used by VeloDB for deployment."
  type        = string

  validation {
    condition     = can(regex("^arn:[^:]+:iam::[0-9]{12}:role/.+$", var.deployment_credential_arn))
    error_message = "deployment_credential_arn must be an IAM role ARN."
  }
}

variable "compute_vcpu" {
  description = "vCPUs for the initial and optional second cluster."
  type        = number
  default     = 4

  validation {
    condition     = var.compute_vcpu >= 4
    error_message = "compute_vcpu must be at least 4."
  }
}

variable "cache_gb" {
  description = "Cache size in GB for the initial and optional second cluster."
  type        = number
  default     = 100

  validation {
    condition     = var.cache_gb >= 100
    error_message = "cache_gb must be at least 100."
  }
}

variable "create_second_cluster" {
  description = "Create an additional compute cluster."
  type        = bool
  default     = false
}
