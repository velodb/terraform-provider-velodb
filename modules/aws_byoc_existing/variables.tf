variable "region" {
  description = "AWS region containing the existing BYOC infrastructure."
  type        = string
  nullable    = false

  validation {
    condition     = var.region != "" && var.region == trimspace(var.region)
    error_message = "region must not be empty or contain leading or trailing whitespace."
  }
}

variable "bucket_name" {
  description = "Existing S3 bucket name. This module does not manage or delete it."
  type        = string
  nullable    = false

  validation {
    condition = (
      can(regex("^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$", var.bucket_name)) &&
      !strcontains(var.bucket_name, "..") &&
      !can(regex("^[0-9]{1,3}(\\.[0-9]{1,3}){3}$", var.bucket_name)) &&
      !can(regex("^(xn--|sthree-|amzn-s3-demo-)", var.bucket_name)) &&
      !can(regex("(-s3alias|--ol-s3|\\.mrap|--x-s3|--table-s3)$", var.bucket_name))
    )
    error_message = "bucket_name must be a valid S3 bucket name: 3–63 lowercase letters, numbers, periods, or hyphens; start and end with a letter or number; no adjacent periods, IP address format, or AWS reserved prefixes/suffixes."
  }
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
  nullable    = false

  validation {
    condition = (
      can(regex("^[A-Za-z0-9!@#$%^&*()_+{}\\[\\]:;<>,.?~\\\\-]{8,20}$", var.admin_password)) &&
      length([for pattern in ["[A-Z]", "[a-z]", "[0-9]", "[^A-Za-z0-9]"] : pattern if can(regex(pattern, var.admin_password))]) >= 3
    )
    error_message = "admin_password must be 8–20 characters using only letters, digits, and !@#$%^&*()_+{}[]:;<>,.?~\\-; include at least three of uppercase letters, lowercase letters, digits, and special characters."
  }
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
  nullable    = false

  validation {
    condition     = var.endpoint_id != "" && var.endpoint_id == trimspace(var.endpoint_id)
    error_message = "endpoint_id must not be empty or contain leading or trailing whitespace."
  }
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
  description = "Legacy vCPUs for the initial cluster. Defaults to 4 when no node settings are supplied."
  type        = number
  default     = null

  # Same rule the API enforces for additional clusters; keep the two in sync.
  validation {
    condition     = var.compute_vcpu == null ? true : contains([4, 8, 16], var.compute_vcpu) || (var.compute_vcpu > 16 && var.compute_vcpu % 16 == 0)
    error_message = "compute_vcpu must be 4, 8, 16, or a multiple of 16 greater than 16."
  }
}

variable "cache_gb" {
  description = "Legacy cache GB for the initial cluster. Defaults to 100 when no node settings are supplied."
  type        = number
  default     = null

  # Baseline floor only. The full rule (cache_gb >= max(100, compute_vcpu * 25))
  # is cross-variable, which variable validation cannot express before Terraform
  # 1.9; it is enforced as a precondition on velodb_warehouse.this in main.tf.
  validation {
    condition     = var.cache_gb == null ? true : var.cache_gb >= 100
    error_message = "cache_gb must be at least 100."
  }
}

variable "ratio" {
  description = "Initial cluster memory GB per vCPU: 4 or 8. Creation-only; null uses the API default."
  type        = number
  default     = null
  validation {
    condition     = var.ratio == null ? true : contains([4, 8], var.ratio)
    error_message = "ratio must be 4 or 8."
  }
}

variable "cache_type" {
  description = "Initial cluster cache type: 0 for EBS (default), 1 for local NVMe. Creation-only; omit node_cache for local storage."
  type        = number
  default     = null
  validation {
    condition     = var.cache_type == null ? true : contains([0, 1], var.cache_type)
    error_message = "cache_type must be 0 (EBS) or 1 (local NVMe)."
  }
}

variable "node_vcpu" {
  description = "vCPUs per node. Set node_vcpu/node_count and omit legacy compute_vcpu/cache_gb. EBS also requires node_cache; omit it for local NVMe. Creation-only for the initial cluster."
  type        = number
  default     = null
  validation {
    condition     = var.node_vcpu == null ? true : var.node_vcpu >= 1 && var.node_vcpu <= 2147483647 && floor(var.node_vcpu) == var.node_vcpu
    error_message = "node_vcpu must be a positive integer no greater than 2147483647."
  }
}

variable "node_count" {
  description = "Initial node count. Set node_vcpu/node_count and omit legacy compute_vcpu/cache_gb. EBS also requires node_cache; omit it for local NVMe. Creation-only for the initial cluster."
  type        = number
  default     = null
  validation {
    condition     = var.node_count == null ? true : var.node_count >= 1 && var.node_count <= 2147483647 && floor(var.node_count) == var.node_count
    error_message = "node_count must be a positive integer no greater than 2147483647."
  }
}

variable "node_cache" {
  description = "Cache GB per node. Set node_vcpu/node_count and omit legacy compute_vcpu/cache_gb. EBS also requires node_cache; omit it for local NVMe. Creation-only for the initial cluster."
  type        = number
  default     = null
  validation {
    condition     = var.node_cache == null ? true : var.node_cache >= 1 && var.node_cache <= 2147483647 && floor(var.node_cache) == var.node_cache
    error_message = "node_cache must be a positive integer no greater than 2147483647."
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

variable "core_version" {
  description = "Desired warehouse version. Creation accepts major.minor or major.minor.patch; upgrades require an exact major.minor.patch. Leave null for the API default."
  type        = string
  default     = null

  validation {
    condition     = var.core_version == null || can(regex("^[0-9]+\\.[0-9]+(\\.[0-9]+)?$", var.core_version))
    error_message = "core_version must use major.minor or major.minor.patch numeric format."
  }
}

variable "table_name_case_sensitive" {
  description = "Whether warehouse table names are case-sensitive. Create-only; leave null to use the case-sensitive server default."
  type        = bool
  default     = null
}

variable "public_access_policy" {
  description = "Public access policy applied at creation and updated in place afterward. Null leaves the policy unmanaged without changing it. rules apply only when policy is ALLOWLIST_ONLY."
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

variable "additional_clusters" {
  description = "Additional compute clusters keyed by a stable Terraform identifier."
  type = map(object({
    name         = string
    zone         = optional(string)
    compute_vcpu = optional(number)
    cache_gb     = optional(number)
    node_vcpu    = optional(number)
    node_count   = optional(number)
    node_cache   = optional(number)
    cache_type   = optional(number)
    ratio        = optional(number)
    scaling_mode = optional(string)
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
      cluster.compute_vcpu == null ? true : contains([4, 8, 16], cluster.compute_vcpu) || (cluster.compute_vcpu > 16 && cluster.compute_vcpu % 16 == 0)
    ])
    error_message = "Each additional cluster compute_vcpu must be 4, 8, 16, or a multiple of 16 greater than 16."
  }

  validation {
    condition = alltrue([
      for cluster in values(var.additional_clusters) :
      cluster.compute_vcpu == null || cluster.cache_gb == null ? true : cluster.cache_gb >= max(100, cluster.compute_vcpu * 25)
    ])
    error_message = "Each additional cluster cache_gb must be at least max(100, compute_vcpu * 25)."
  }
  validation {
    condition = alltrue([
      for cluster in values(var.additional_clusters) :
      (cluster.cache_type == 1 || cluster.node_vcpu != null || cluster.node_count != null || cluster.node_cache != null) ?
      (cluster.compute_vcpu == null && cluster.cache_gb == null && alltrue([for v in concat([cluster.node_vcpu, cluster.node_count], cluster.cache_type == 1 ? [] : [cluster.node_cache]) : v == null ? false : v >= 1 && v <= 2147483647 && floor(v) == v]) && (cluster.cache_type == 1 ? cluster.node_cache == null : true)) :
      (cluster.compute_vcpu != null && cluster.cache_gb != null)
    ])
    error_message = "Each additional cluster requires legacy compute_vcpu/cache_gb or node_vcpu/node_count with node_cache for EBS. Omit node_cache for local NVMe; do not mix formats."
  }
  validation {
    condition     = alltrue([for cluster in values(var.additional_clusters) : cluster.ratio == null ? true : contains([4, 8], cluster.ratio)])
    error_message = "Each additional cluster ratio must be 4 or 8."
  }
  validation {
    condition     = alltrue([for cluster in values(var.additional_clusters) : cluster.cache_type == null ? true : contains([0, 1], cluster.cache_type)])
    error_message = "Each additional cluster cache_type must be 0 (EBS) or 1 (local NVMe)."
  }
  validation {
    condition     = alltrue([for cluster in values(var.additional_clusters) : cluster.scaling_mode == null ? true : cluster.scaling_mode == "MANUAL" && cluster.node_vcpu != null])
    error_message = "scaling_mode supports MANUAL only with node-based capacity. Cache-only mode switches are rejected by the provider."
  }
}

variable "create_tde_encryption_key" {
  description = "Create a new AWS KMS key for warehouse transparent data encryption (TDE) and register it with VeloDB. Mutually exclusive with tde_kms_key_arn."
  type        = bool
  default     = false
}

variable "tde_kms_key_arn" {
  description = "ARN of an existing AWS KMS key to register for warehouse TDE. Mutually exclusive with create_tde_encryption_key. Leave null to not use a customer-managed TDE key."
  type        = string
  default     = null
}

variable "create_ebs_encryption_key" {
  description = "Create a new AWS KMS key for warehouse EBS volume encryption and register it with VeloDB. Mutually exclusive with ebs_kms_key_arn."
  type        = bool
  default     = false
}

variable "ebs_kms_key_arn" {
  description = "ARN of an existing AWS KMS key to register for warehouse EBS encryption. Mutually exclusive with create_ebs_encryption_key. Leave null to not use a customer-managed EBS key."
  type        = string
  default     = null
}

variable "kms_key_deletion_window_in_days" {
  description = "Deletion window for KMS keys created by this module (create_tde_encryption_key / create_ebs_encryption_key)."
  type        = number
  default     = 30

  validation {
    condition     = var.kms_key_deletion_window_in_days >= 7 && var.kms_key_deletion_window_in_days <= 30
    error_message = "kms_key_deletion_window_in_days must be between 7 and 30."
  }
}
