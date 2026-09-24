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
