# AWS policy data-source live test

This standalone configuration tests the provider without `byoc-terraform`.
It reads live VeloDB prerequisites, generates all four AWS policy documents,
and asks AWS to create disposable IAM resources from them. It does not create
S3 buckets, networking, NAT gateways, KMS keys, or warehouses.

Build the provider and enable the local development override:

```bash
cd /path/to/terraform-provider-velodb
go build -o /private/tmp/velodb-provider-bin/terraform-provider-velodb
export TF_CLI_CONFIG_FILE=/private/tmp/velodb-policy-terraformrc
export VELODB_API_KEY='replace-me'
export AWS_PROFILE='replace-with-test-profile'
```

Initialize and review the live plan:

```bash
terraform -chdir=test/aws_policies init
terraform -chdir=test/aws_policies plan \
  -var='region=us-east-1' \
  -var='bucket_name=replace-with-a-valid-test-bucket-name' \
  -out=/private/tmp/velodb-aws-policy-live.tfplan
terraform -chdir=test/aws_policies show /private/tmp/velodb-aws-policy-live.tfplan
```

The plan calls AWS STS and the VeloDB prerequisites API but changes nothing.
Applying the saved plan creates only the IAM resources listed above:

```bash
terraform -chdir=test/aws_policies apply /private/tmp/velodb-aws-policy-live.tfplan
```

Destroy them after checking the outputs:

```bash
terraform -chdir=test/aws_policies destroy \
  -var='region=us-east-1' \
  -var='bucket_name=replace-with-a-valid-test-bucket-name'
```
