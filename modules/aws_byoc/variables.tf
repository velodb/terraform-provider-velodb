variable "region" {
  description = "AWS region supported by VeloDB BYOC."
  type        = string
}

variable "bucket_name" {
  description = "Globally unique S3 bucket name for warehouse data."
  type        = string
}

variable "name_prefix" {
  description = "Prefix for AWS and VeloDB resources. Do not change after creation."
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

variable "zones" {
  description = "One zone for single-zone deployment or three zones for cross-zone deployment."
  type        = list(string)

  validation {
    condition     = contains([1, 3], length(var.zones)) && length(distinct(var.zones)) == length(var.zones)
    error_message = "Provide exactly one zone for single-zone deployment or three distinct zones for cross-zone deployment."
  }
}

variable "vpc_cidr" {
  description = "CIDR block for the new VPC."
  type        = string
  default     = "10.57.0.0/16"

  validation {
    condition     = can(cidrhost(var.vpc_cidr, 0))
    error_message = "vpc_cidr must be a valid IPv4 CIDR block, for example \"10.57.0.0/16\"."
  }
}

variable "subnet_cidrs" {
  description = "Optional availability-zone to subnet CIDR overrides. Each key must be one of the configured zones; unset zones fall back to a /20 block derived from the AZ letter. Pin an existing deployment's current CIDRs here before upgrading to avoid a subnet/network/warehouse replacement, or place subnets for non-standard zones or an external network plan."
  type        = map(string)
  default     = {}

  validation {
    condition     = alltrue([for cidr in values(var.subnet_cidrs) : can(cidrhost(cidr, 0))])
    error_message = "Each subnet_cidrs value must be a valid IPv4 CIDR block, for example \"10.57.16.0/20\"."
  }
}

variable "warehouse_client_cidrs" {
  description = "CIDR blocks of VPCs that need to reach the warehouse on ports 8000-10000. Leave empty when access is only via PrivateLink."
  type        = list(string)
  default     = []

  validation {
    condition     = alltrue([for cidr in var.warehouse_client_cidrs : can(cidrhost(cidr, 0))])
    error_message = "Each warehouse_client_cidrs value must be a valid IPv4 CIDR block, for example \"10.0.0.0/16\"."
  }

  validation {
    # Reject /0 (e.g. 0.0.0.0/0), which would open the warehouse ports to the
    # entire internet. Scope access to the specific client VPC CIDRs instead.
    condition     = alltrue([for cidr in var.warehouse_client_cidrs : can(cidrhost(cidr, 0)) ? tonumber(split("/", cidr)[1]) > 0 : true])
    error_message = "warehouse_client_cidrs must not include a /0 block such as 0.0.0.0/0; specify the client VPC CIDRs that need warehouse access."
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

variable "tags" {
  description = "Additional tags for AWS resources and the VeloDB warehouse."
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

  validation {
    condition = var.public_access_policy == null || alltrue([
      for r in try(var.public_access_policy.rules, []) : can(cidrhost(r.cidr, 0))
    ])
    error_message = "Each public_access_policy.rules[].cidr must be a valid IPv4 CIDR block, for example \"203.0.113.0/24\"."
  }
}
