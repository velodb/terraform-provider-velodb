# Full AWS BYOC module live test

This root configuration calls the reusable `modules/aws_byoc` module. It is
independent of `byoc-terraform` and exercises the same short workflow intended
for users.

The module creates a new VPC, one public subnet, three private subnets, a NAT
gateway and Elastic IP, routing, IAM, S3, security groups, PrivateLink, VeloDB
registrations, and a warehouse. These resources incur charges until destroyed.

## Environment

```bash
export AWS_PROFILE='replace-with-test-profile'
export VELODB_HOST='sandbox-api.velodb.io'
export VELODB_API_KEY='replace-with-sandbox-key'
export TF_VAR_admin_password='replace-with-a-strong-test-password'
export TF_CLI_CONFIG_FILE=/private/tmp/velodb-policy-terraformrc

go build -o /private/tmp/velodb-provider-bin/terraform-provider-velodb
cp test/aws_byoc_full/live.tfvars.example test/aws_byoc_full/live.auto.tfvars
terraform -chdir=test/aws_byoc_full init
```

The copied `live.auto.tfvars` is ignored by Git.

## Create and verify

```bash
terraform -chdir=test/aws_byoc_full plan -out=tfplan
terraform -chdir=test/aws_byoc_full apply tfplan
terraform -chdir=test/aws_byoc_full plan
```

The first plan creates the complete deployment. The final plan must report
`No changes`.

## Destroy and verify

```bash
terraform -chdir=test/aws_byoc_full plan -destroy -out=destroy.tfplan
terraform -chdir=test/aws_byoc_full apply destroy.tfplan
terraform -chdir=test/aws_byoc_full state list
```

The final command must return no resources. Terraform deletes the warehouse
before its VeloDB registrations and AWS dependencies. If warehouse deletion
cannot be confirmed, destroy stops and preserves state.
