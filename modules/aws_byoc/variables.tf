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

variable "additional_clusters" {
  description = "Additional compute clusters keyed by a stable Terraform identifier."
  type = map(object({
    name         = string
    zone         = optional(string)
    compute_vcpu = number
    cache_gb     = number
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

variable "bucket_force_destroy" {
  description = "Delete objects with the bucket during destroy. Keep false for production."
  type        = bool
  default     = false
}

variable "tags" {
  description = "Additional tags for AWS resources."
  type        = map(string)
  default     = {}
}
