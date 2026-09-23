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
  description = "Exactly three VeloDB-supported availability zones."
  type        = list(string)

  validation {
    condition     = length(var.zones) == 3 && length(distinct(var.zones)) == 3
    error_message = "Exactly three distinct availability zones are required."
  }
}

variable "vpc_cidr" {
  description = "CIDR block for the new VPC."
  type        = string
  default     = "10.57.0.0/16"
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
