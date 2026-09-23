# AWS BYOC existing-infrastructure module

Creates VeloDB credential and network registrations plus a warehouse using
existing AWS infrastructure. AWS resources are read for validation only; this
module does not create, import, modify, or delete them.

The module verifies the bucket, VPC, subnets, security group, endpoint, IAM
instance profile, and IAM role before its first VeloDB create request.

Do not run `terraform import` for shared AWS resources in this warehouse state.
Import transfers lifecycle management to Terraform and makes a resource subject
to modification, replacement, and deletion.

Required existing infrastructure:

- An S3 bucket.
- A VPC with DNS support and DNS hostnames enabled.
- Three private subnets in VeloDB-supported availability zones, each with a
  default route through an available NAT gateway.
- A warehouse security group.
- A VeloDB interface VPC endpoint in the same VPC.
- A data-access IAM instance profile and deployment IAM role with the required
  VeloDB policies.

Destroy removes the warehouse and its VeloDB registrations. All AWS resources
remain and must be removed separately by their owner if no longer needed.
