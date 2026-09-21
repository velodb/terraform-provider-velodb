# Full AWS BYOC live test

This configuration is independent of `byoc-terraform`. It creates disposable
AWS IAM, storage, security, and PrivateLink resources; registers them with
VeloDB; creates a BYOC warehouse; and can add a second cluster.

The test reuses an existing three-zone VPC because it does not create a VPC,
subnets, routes, NAT gateways, Elastic IPs, or an S3 gateway endpoint. Verify
that every selected private subnet has working outbound connectivity before
running the test. The warehouse and test-created resources can incur charges
until destroyed.

## 1. Environment

Run these commands from the provider repository:

```bash
export AWS_PROFILE='replace-with-test-profile'
export VELODB_HOST='sandbox-api.velodb.io'
export VELODB_API_KEY='replace-with-sandbox-key'
export TF_VAR_admin_password='replace-with-a-strong-test-password'
export TF_CLI_CONFIG_FILE=/private/tmp/velodb-policy-terraformrc

go build -o /private/tmp/velodb-provider-bin/terraform-provider-velodb
cp test/aws_byoc_full/live.tfvars.example test/aws_byoc_full/live.auto.tfvars
# Edit live.auto.tfvars with disposable names and existing test-network IDs.
terraform -chdir=test/aws_byoc_full init
```

The copied `live.auto.tfvars` is ignored by Git.

## 2. Phase one: AWS prerequisites

```bash
terraform -chdir=test/aws_byoc_full plan \
  -out=/private/tmp/velodb-byoc-aws.tfplan

terraform -chdir=test/aws_byoc_full apply \
  /private/tmp/velodb-byoc-aws.tfplan
```

The plan should create 17 AWS resources and no VPC, subnet, route, NAT gateway,
or Elastic IP. This phase must show null VeloDB IDs in `test_status`. Allow time
for AWS IAM permissions to propagate before phase two.

## 3. Phase two: registrations and warehouse

```bash
terraform -chdir=test/aws_byoc_full plan \
  -var='create_velodb_resources=true' \
  -out=/private/tmp/velodb-byoc-warehouse.tfplan

terraform -chdir=test/aws_byoc_full apply \
  /private/tmp/velodb-byoc-warehouse.tfplan
```

The output must contain non-null credential, network, warehouse, and initial
cluster IDs. Run the same plan command again and require `No changes`.

## 4. Phase three: second cluster

```bash
terraform -chdir=test/aws_byoc_full plan \
  -var='create_velodb_resources=true' \
  -var='create_second_cluster=true' \
  -out=/private/tmp/velodb-byoc-cluster.tfplan

terraform -chdir=test/aws_byoc_full apply \
  /private/tmp/velodb-byoc-cluster.tfplan
```

Run the same plan command again and require `No changes`.

## 5. Destroy all test-created resources

```bash
terraform -chdir=test/aws_byoc_full destroy \
  -var='create_velodb_resources=true' \
  -var='create_second_cluster=true'
```

Terraform must fully delete clusters and the warehouse before deleting BYOC
registrations and their AWS dependencies. If warehouse deletion cannot be
confirmed, destroy stops and retains state. The existing VPC, subnets, routes,
NAT gateways, and S3 endpoint remain untouched.
