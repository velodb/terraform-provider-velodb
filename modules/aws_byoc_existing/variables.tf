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
  description = "One or three existing private subnet IDs keyed by VeloDB-supported availability zone."
  type        = map(string)

  validation {
    condition     = contains([1, 3], length(var.subnet_ids_by_zone)) && length(distinct(values(var.subnet_ids_by_zone))) == length(var.subnet_ids_by_zone)
    error_message = "Provide exactly one subnet for single-zone deployment or three distinct subnets for cross-zone deployment, keyed by availability zone."
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
  description = "vCPUs for the initial cluster."
  type        = number
  default     = 4

  validation {
    condition     = var.compute_vcpu >= 4
    error_message = "compute_vcpu must be at least 4."
  }
}

variable "cache_gb" {
  description = "Cache size in GB for the initial cluster."
  type        = number
  default     = 100

  validation {
    condition     = var.cache_gb >= 100
    error_message = "cache_gb must be at least 100."
  }
}

variable "auto_pause" {
  description = "Auto-pause for the initial cluster. Set enabled = true with idle_timeout_minutes to pause the cluster after that many idle minutes."
  type = object({
    enabled              = bool
    idle_timeout_minutes = optional(number)
  })
  default = {
    enabled = false
  }
}

variable "tags" {
  description = "Additional tags for the VeloDB warehouse."
  type        = map(string)
  default     = {}
}

variable "initial_core_version" {
  description = "Initial core version to provision, in major.minor numeric format (e.g. 26.1). Create-only; leave null to let the management API pick the default."
  type        = string
  default     = null

  validation {
    condition     = var.initial_core_version == null || can(regex("^[0-9]+\\.[0-9]+$", var.initial_core_version))
    error_message = "initial_core_version must use major.minor numeric format (e.g. 26.1)."
  }
}

variable "public_access_policy" {
  description = "Initial public access policy applied at warehouse creation. Create-only; leave null to let the management API pick the default. Manage it afterward with the velodb_warehouse_public_access_policy resource. rules apply only when policy is ALLOWLIST_ONLY."
  type = object({
    policy = string
    rules = optional(list(object({
      cidr        = string
      description = optional(string)
    })), [])
  })
  default = null

  validation {
    condition     = var.public_access_policy == null || contains(["DENY_ALL", "ALLOW_ALL", "ALLOWLIST_ONLY"], try(var.public_access_policy.policy, ""))
    error_message = "public_access_policy.policy must be one of DENY_ALL, ALLOW_ALL, or ALLOWLIST_ONLY."
  }

  validation {
    condition     = var.public_access_policy == null || try(var.public_access_policy.policy, "") == "ALLOWLIST_ONLY" || length(try(var.public_access_policy.rules, [])) == 0
    error_message = "public_access_policy.rules may only be set when policy is ALLOWLIST_ONLY."
  }
}

variable "additional_clusters" {
  description = "Additional compute clusters keyed by a stable Terraform identifier."
  type = map(object({
    name         = string
    zone         = optional(string)
    compute_vcpu = number
    cache_gb     = number
    auto_pause = optional(object({
      enabled              = bool
      idle_timeout_minutes = optional(number)
    }), { enabled = false })
  }))
  default = {}

  validation {
    condition = alltrue([
      for cluster in values(var.additional_clusters) :
      can(regex("^[A-Za-z][A-Za-z0-9_]{0,31}$", cluster.name))
    ])
    error_message = "Each additional cluster name must start with a letter, contain only letters, numbers, or underscores, and be at most 32 characters."
  }

  validation {
    condition     = length(distinct([for cluster in values(var.additional_clusters) : cluster.name])) == length(var.additional_clusters)
    error_message = "Each additional cluster must have a unique name."
  }

  validation {
    condition = alltrue([
      for cluster in values(var.additional_clusters) :
      contains([4, 8, 16], cluster.compute_vcpu) || (cluster.compute_vcpu > 16 && cluster.compute_vcpu % 16 == 0)
    ])
    error_message = "Each additional cluster compute_vcpu must be 4, 8, 16, or a multiple of 16 greater than 16."
  }

  validation {
    condition = alltrue([
      for cluster in values(var.additional_clusters) :
      cluster.cache_gb >= max(100, cluster.compute_vcpu * 25)
    ])
    error_message = "Each additional cluster cache_gb must be at least max(100, compute_vcpu * 25)."
  }
}
