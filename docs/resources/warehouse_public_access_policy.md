---
page_title: "velodb_warehouse_public_access_policy Resource - velodb"
subcategory: ""
description: |-
  Deprecated compatibility resource. Use velodb_warehouse.public_access_policy.
---

# velodb_warehouse_public_access_policy (Deprecated)

Use the `public_access_policy` block in [`velodb_warehouse`](warehouse.md#public-access-policy)
for all new configurations. That single block manages creation-time policy,
in-place updates, and drift detection. The standalone resource remains registered
only to support existing Terraform state and emits a deprecation warning.

## Migrate existing state

1. Ensure the warehouse is managed by a `velodb_warehouse` resource. Import an
   existing warehouse if needed, and verify its configuration matches the existing
   deployment before proceeding.
2. Copy the current policy and rules into that warehouse's `public_access_policy`
   block (or the BYOC module input of the same name).
3. Remove the standalone resource from Terraform state **without destroying it**,
   using its actual address, for example:

   ```sh
   terraform state rm velodb_warehouse_public_access_policy.legacy
   ```

   Alternatively, use a Terraform `removed` block with `destroy = false` on
   versions that support it.
4. Remove the standalone resource configuration. Do not apply an intermediate
   configuration that manages the policy through both forms.
5. Run `terraform plan`. With matching policy values, this migration must not
   change remote access or replace the warehouse. Review the plan before applying.

Do not migrate by simply deleting the standalone resource configuration and
applying: its delete operation resets remote access to `DENY_ALL`.

## Legacy schema

- `warehouse_id` (required string): warehouse ID; changing it replaces this policy
  resource, not the warehouse.
- `policy` (required string): `DENY_ALL`, `ALLOW_ALL`, or `ALLOWLIST_ONLY`.
- `rules` (optional set): allowlist entries with required `cidr` and optional
  `description`; valid only for `ALLOWLIST_ONLY`.
- `id` (computed string): same as `warehouse_id`.

Legacy destruction resets the policy to `DENY_ALL`. Legacy import uses the
warehouse ID. These behaviors remain for compatibility, not as the recommended
management workflow.
