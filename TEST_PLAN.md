# Terraform Provider Sandbox Test Plan

Goal: validate the Terraform provider against the current Management API YAML in
`/Users/zhhanz/Documents/velodb/Skills/managementapi(1).yaml`.

## API Shape Covered

- Warehouse create uses `deploymentMode = "SaaS"` or `"BYOC"`.
- Warehouse create sends only API-supported fields: `name`, `deploymentMode`,
  `cloudProvider`, `region`, `vpcMode`, `setupMode`, `credentialId`,
  `networkConfigId`, `adminPassword`, and `initialCluster`.
- Warehouse upgrade uses `POST /v1/warehouses/{warehouseId}/settings/upgrade`
  with `targetVersionId`.
- Cluster actions use explicit endpoints:
  `/pause`, `/resume`, and `/reboot`.
- Cluster create/update sends `name`, `clusterType`, `zone`, `computeVcpu`,
  `cacheGb`, and `autoPause`; billing selection fields are not sent.
- Connections are read from `GET /v1/warehouses/{warehouseId}/connections`.
- Warehouse private endpoint registration uses
  `POST /v1/private-link/warehouses/{warehouseId}/endpoints`.

## Local Verification

1. `go test ./...`
2. `go install .`
3. `terraform -chdir=test/sandbox/phase1 validate -no-color` with a dev override
   for `velodb/velodb`.
4. `bash test/sandbox/scripts/phase7.sh` with the same dev override.

## Live Sandbox Phases

Run against `sandbox-api.velodb.io` with `TF_VAR_api_key` set.

| Phase | Purpose |
|---|---|
| 1 | Create warehouse, read versions data source, assert clean base shape |
| 2 | Rename, password rotation, invalid `core_version_id` handling |
| 3 | Cluster pause/resume/reboot through explicit endpoints |
| 4 | Cluster CPU resize, then cache resize |
| 5 | Edge cases: invalid version, invalid vCPU, invalid cache, auto-pause timeout validation, resize while paused |
| 6 | Import warehouse and cluster; assert no drift on read-back fields |
| 7 | Validate removed/stale fields fail loudly without API calls |

## Known API Limits

- `maintenance_window` and `upgrade_policy` are not in the current create/update
  API and should fail as unsupported Terraform configuration.
- Mixed billing create/update fields are not in the current cluster request
  schema. `billing_model` is exposed only as observed data.
- PrivateLink endpoint deregistration is not exposed by the current API. Destroy
  removes the Terraform resource from state only.
