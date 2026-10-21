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
- One private subnet for single-zone deployment or three private subnets for
  cross-zone deployment. Each must be in a VeloDB-supported availability zone
  with a default route through an available NAT gateway.
- A warehouse security group.
- A VeloDB interface VPC endpoint in the same VPC.
- A data-access IAM instance profile and deployment IAM role with the required
  VeloDB policies.

Create any number of additional clusters with a stable map key:

```hcl
additional_clusters = {
  analytics = {
    name         = "production_analytics"
    compute_vcpu = 8
    cache_gb     = 200
  }
}
```

`zone` is optional and defaults to the first configured zone. Change `name` to
rename a cluster without changing its Terraform identity. Removing an entry
deletes only that additional cluster.

`initial_cluster_id` is captured on a best-effort basis during warehouse creation
and remains null after importing an existing warehouse. Refresh does not infer
it from current clusters. Warehouse import does not import physical clusters.
To manage an existing compute cluster, import it separately as a `velodb_cluster`
resource. Virtual-cluster members can use managed resource IDs, explicit
[`velodb_clusters` data-source lookups](../../docs/data-sources/clusters.md),
or verified physical IDs without importing the physical resources.
Verify older cached IDs before using them for imports or pair membership.
See [Managing the Initial Cluster](../../docs/resources/warehouse.md#managing-the-initial-cluster).

Create virtual clusters as separate `velodb_virtual_cluster` resources linked
to this module's output IDs. Terraform resolves these IDs during apply, so the
warehouse, physical clusters, and virtual cluster can be created together. The
two members are any two distinct compute clusters in the warehouse; the active
and standby roles are independent of which one is the initial cluster. Create a
physical standby cluster in another zone and reference its ID:

```hcl
resource "velodb_cluster" "standby" {
  warehouse_id = module.velodb_byoc.warehouse_id
  name         = "production_standby"
  cluster_type = "COMPUTE"
  zone         = "us-east-1b"
  compute_vcpu = 4
  cache_gb     = 100

  lifecycle {
    create_before_destroy = true
  }
}

resource "velodb_virtual_cluster" "main" {
  warehouse_id = module.velodb_byoc.warehouse_id
  name         = "production_vc"

  # Either ID can be any compute cluster in the warehouse. Here the module's
  # initial cluster is active and a separately managed cluster is standby.
  desired_active_cluster_id  = module.velodb_byoc.initial_cluster_id
  desired_standby_cluster_id = velodb_cluster.standby.id
}
```

These inputs also accept IDs from separately managed resources or existing
clusters. Change either desired ID to update the pair. Live roles and health
are read from the virtual-cluster resource; automatic failover does not
trigger a reversal to the configured pair. Change `pair_reconcile_trigger`
to explicitly reapply that pair.

Additional clusters managed by this module use `create_before_destroy` so a
replacement can be created and attached to the virtual cluster before the old
member is deleted. Standalone members need the lifecycle rule shown above.
Existing deployments must first apply the updated module or lifecycle rule
while keeping all current member declarations and pair IDs unchanged. This
records the rule in state before an outgoing resource or map key is removed.
A replacement needs a distinct name (ignoring case), spare capacity, and a zone
different from its partner. Renaming a cluster alone does not replace it.

If a bound-member deletion already failed, restore the old declaration, apply
the pair update, then remove the old member in a later apply. See
[Replacing physical members](../../docs/resources/virtual_cluster.md#replacing-physical-members)
for the replacement and recovery procedure. Deleting the virtual-cluster
resource ends its routing and automatic failover while leaving its physical
clusters in place.

If adopting this configuration from the earlier bundled virtual-cluster
implementation, move its Terraform state to the separate resource before
applying; see the virtual-cluster resource documentation. Use a provider
release that includes `velodb_virtual_cluster` for the separate resource.

Set `tags` to apply extra key/value pairs to the VeloDB warehouse (it gets a
`managed-by = terraform` tag by default). Warehouse tags update in place and
serve as defaults for existing and future clusters. Organization tags take
precedence over cluster overrides, which take precedence over warehouse
defaults. Refresh reads the stored warehouse tags, excluding inherited tags and
direct AWS edits. Set `core_version` to `"26.1"` (backend selects the latest patch) or `"26.1.1"`
at creation. Change it to an exact three-part version such as `"26.1.2"` to
upgrade the same warehouse in place. A two-part creation selector remains
two-part in Terraform state; the provider reads the actual patch internally
before an upgrade.

Set `table_name_case_sensitive` to `false` for case-insensitive table names.
It is create-only; leave it unset to use the case-sensitive server default.

Set `public_access_policy` to manage public access at creation and afterward: an
object with `policy` (`DENY_ALL`, `ALLOW_ALL`, or `ALLOWLIST_ONLY`) and, for
`ALLOWLIST_ONLY`, a list of `rules` (`cidr` plus optional `description`). Changes update the existing warehouse in
place, and refresh detects remote
policy/rule changes. Leave it unset to use the API default at creation. Setting
it to null later stops management without changing the remote policy; use
`DENY_ALL` to disable public access. Do not also manage the same policy with
`velodb_warehouse_public_access_policy`. For example:

```hcl
public_access_policy = {
  policy = "ALLOWLIST_ONLY"
  rules = [
    { cidr = "203.0.113.0/24", description = "office" },
  ]
}
```

Destroy removes the warehouse and its VeloDB registrations. All AWS resources
remain and must be removed separately by their owner if no longer needed.

## S3 bucket name validation

`bucket_name` must be 3–63 characters using lowercase letters, numbers, periods,
or hyphens, and must start and end with a letter or number. Adjacent periods,
IP address formats, and AWS reserved prefixes/suffixes are rejected. For example,
`velodb-data` is valid; `-velodb-data` is rejected before infrastructure changes.
See the [AWS bucket naming rules](https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html).

The provider validates known bucket names during `terraform validate` and the
configuration validation phase of `terraform plan`/`apply`. Both BYOC modules
also validate the input variable when its value is known. Values derived from
other resources are checked once Terraform resolves them. This syntax check
does not verify bucket availability, ownership, permissions, or namespace.

## Immutable warehouse infrastructure

Once created, warehouse infrastructure inputs cannot be edited in place. The
module rejects changes to `bucket_name`, `region`, network placement, and
`create_tde_encryption_key`, `create_ebs_encryption_key`, `tde_kms_key_arn`, and
`ebs_kms_key_arn`. The new-VPC module also freezes `vpc_cidr`, `zones`, and
`subnet_cidrs`; the existing-infrastructure module freezes VPC, subnet, security
group, endpoint, and IAM credential references. Restore the original values
when a plan reports an immutable-input error. Provision a separate warehouse
for a migration instead of replacing the existing warehouse through input edits.

When upgrading a deployment created before these guards existed, first apply
with all infrastructure inputs unchanged to record their baseline. Do not
combine that upgrade with infrastructure edits. The provider also rejects
changes to existing warehouse bindings, including unknown IDs produced by
upstream replacement plans. An unknown binding must be resolved without
replacing infrastructure already used by the warehouse before planning again.

These checks do not prevent an explicit `terraform destroy` or removal of the
module from configuration. They are immutability checks, not deletion protection.

Warehouse refresh and import recover `credential_id` and `network_config_id` when
the backend returns `credentialId` and `networkConfigId`. Older backends retain
known bindings in state. Changes to known bindings are rejected without replacing
the warehouse.

## Input validation

Administrator passwords must be 8–20 characters, contain at least three of
uppercase letters, lowercase letters, digits, and special characters, and use
only ASCII letters, digits, and these special characters:

```text
!@#$%^&*()_+{}[]:;<>,.?~\-
```

Known invalid passwords fail configuration validation without including the
password in error messages. Both BYOC modules require a valid password for warehouse creation.

`region` must not be empty or contain leading or trailing whitespace.
The required existing `endpoint_id` must also be nonempty with no leading or
trailing whitespace.
