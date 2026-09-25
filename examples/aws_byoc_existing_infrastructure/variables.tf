variable "region" {
  description = "AWS region containing the existing BYOC infrastructure."
  type        = string
  default     = "us-east-1"
}

variable "bucket_name" {
  description = "Existing S3 bucket name."
  type        = string
}

variable "name_prefix" {
  description = "Prefix for VeloDB resources. Do not change after creation."
  type        = string
}

variable "admin_password" {
  description = "Warehouse administrator password. Set with TF_VAR_admin_password."
  type        = string
  sensitive   = true
}

variable "vpc_id" {
  description = "Existing VPC ID."
  type        = string
}

variable "subnet_ids_by_zone" {
  description = "One or three existing private subnet IDs keyed by availability zone."
  type        = map(string)
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

variable "security_group_id" {
  description = "Existing warehouse security group ID."
  type        = string
}

variable "endpoint_id" {
  description = "Existing VeloDB interface VPC endpoint ID."
  type        = string
}

variable "data_credential_arn" {
  description = "Existing data-access IAM instance-profile ARN."
  type        = string
}

variable "deployment_credential_arn" {
  description = "Existing VeloDB deployment IAM role ARN."
  type        = string
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
