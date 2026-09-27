variable "region" {
  description = "AWS region supported by VeloDB BYOC."
  type        = string
  default     = "us-east-1"
}

variable "bucket_name" {
  description = "Globally unique S3 bucket name for warehouse data."
  type        = string
}

variable "name_prefix" {
  description = "Prefix for AWS and VeloDB resources. Do not change after creation."
  type        = string
}

variable "admin_password" {
  description = "Warehouse administrator password. Set with TF_VAR_admin_password."
  type        = string
  sensitive   = true
}

variable "zones" {
  description = "One VeloDB-supported zone for single-zone deployment or three for cross-zone deployment."
  type        = list(string)
}

variable "vpc_cidr" {
  description = "CIDR block for the new VPC."
  type        = string
  default     = "10.57.0.0/16"
}

variable "subnet_cidrs" {
  description = "Optional zone to subnet CIDR overrides. Unset zones fall back to a /20 block derived from the AZ letter. Use it for non-standard zones or to pin CIDRs when upgrading an existing deployment."
  type        = map(string)
  default     = {}
}

variable "warehouse_client_cidrs" {
  description = "CIDR blocks allowed to reach warehouse query ports 8000-10000."
  type        = list(string)
  default     = []
}

variable "additional_clusters" {
  description = "Additional compute clusters keyed by a stable Terraform identifier."
  type = map(object({
    name         = string
    zone         = optional(string)
    compute_vcpu = number
    cache_gb     = number
  }))
  default = {}
}

variable "tags" {
  description = "Additional tags for AWS resources."
  type        = map(string)
  default     = {}
}

variable "initial_core_version" {
  description = "Initial core version to provision, in major.minor numeric format (e.g. 26.1). Create-only; leave null to let the management API pick the default."
  type        = string
  default     = null
}

variable "public_access_policy" {
  description = "Initial public access policy applied at warehouse creation. Create-only; leave null to let the management API pick the default. rules apply only when policy is ALLOWLIST_ONLY."
  type = object({
    policy = string
    rules = optional(list(object({
      cidr        = string
      description = optional(string)
    })), [])
  })
  default = null
}

variable "create_tde_encryption_key" {
  description = "Create a new AWS KMS key for warehouse TDE and register it with VeloDB."
  type        = bool
  default     = false
}

variable "tde_kms_key_arn" {
  description = "ARN of an existing AWS KMS key to register for warehouse TDE. Mutually exclusive with create_tde_encryption_key."
  type        = string
  default     = null
}

variable "create_ebs_encryption_key" {
  description = "Create a new AWS KMS key for warehouse EBS encryption and register it with VeloDB."
  type        = bool
  default     = false
}

variable "ebs_kms_key_arn" {
  description = "ARN of an existing AWS KMS key to register for warehouse EBS encryption. Mutually exclusive with create_ebs_encryption_key."
  type        = string
  default     = null
}
