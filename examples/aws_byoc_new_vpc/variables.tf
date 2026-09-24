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
