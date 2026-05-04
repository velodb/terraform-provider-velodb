# Test Plan — v1.0 Migration

Goal: validate the four mandatory API migrations (cluster actions, `targetVersionId`, `maintenance_window` shape, settings response) and the regressions they could cause, without breaking the runtime-only features the new spec dropped (mixed billing, convert-to-subscription, renew, legacy connections endpoints).

Sandbox: `https://sandbox.velodb.io`
Test warehouse on file: `AWVA7G1I` (cli-test, SaaS, on AWS us-east-1).

## Phase 0 — Local (5 min, repeat after every code change)

| Step | Command | Pass criterion |
|---|---|---|
| 0.1 | `go build ./...` | exits 0 |
| 0.2 | `go vet ./...` | no warnings |
| 0.3 | `go test ./...` | all packages OK |
| 0.4 | `go install .` and add `dev_overrides` block to `~/.terraformrc` | `terraform plan` finds the local provider |

## Phase 1 — Warehouse: new HCL shape, fresh resource (~15 min)

Config exercising every migrated field:

```hcl
resource "velodb_warehouse" "t" {
  name            = "tfmig-test"
  deployment_mode = "SaaS"
  cloud_provider  = "aws"
  region          = "us-east-1"
  admin_password  = "Tf@Migration123"

  upgrade_policy = "automatic"
  maintenance_window = {
    start_hour_utc = 3
    end_hour_utc   = 5
  }

  initial_cluster {
    name = "default"
    zone = "us-east-1d"
    compute_vcpu = 4
    cache_gb     = 100
    auto_pause { enabled = false }
  }
}

data "velodb_warehouse_versions" "v" {
  warehouse_id = velodb_warehouse.t.id
}

output "version_default_id" { value = data.velodb_warehouse_versions.v.default_id }
output "version_list"       { value = data.velodb_warehouse_versions.v.versions }
```

| # | Test | Pass criterion |
|---|---|---|
| 1.1 | `terraform apply` | warehouse reaches `Running`, no errors |
| 1.2 | `terraform plan` immediately after apply | **exit 0, no diff** (drift check) |
| 1.3 | `terraform state show velodb_warehouse.t` | shows `core_version` populated, `upgrade_policy="automatic"`, `maintenance_window={3,5}` |
| 1.4 | `terraform output version_list` | array (may be `[]` — that's a sandbox artefact) |
| 1.5 | `terraform refresh && terraform plan` | still no diff |

## Phase 2 — Warehouse mutations (~15 min)

Using the resource from Phase 1.

| # | Mutation | Expected API call | Pass criterion |
|---|---|---|---|
| 2.1 | Change `maintenance_window.start_hour_utc` 3→4 | `PATCH /settings` with `{maintenanceWindow:{startHourUtc:4,endHourUtc:5}}` plus `upgradePolicy:"automatic"` re-sent | apply succeeds, post-plan clean |
| 2.2 | Set only `upgrade_policy` (remove `maintenance_window` block) | `PATCH /settings` body has only `upgradePolicy` — confirms the new "either-or" handling fires correctly | apply succeeds, no 400 |
| 2.3 | Remove BOTH `upgrade_policy` and `maintenance_window` | provider emits warning "Cannot clear both…", **does not call API** | apply succeeds with warning, state retains prior values |
| 2.4 | Set `start_hour_utc = 25` | provider rejects via validator | `terraform plan` fails with `"value must be between 0 and 23"` — never hits API |
| 2.5 | `terraform apply -replace=null_resource.x` is N/A; instead change `name = "tfmig-test-renamed"` | `PATCH /warehouses/{id}` with `{name}` only | apply succeeds, post-plan clean |
| 2.6 | Rotate `admin_password` | `POST /settings/password` | apply succeeds, can log into MySQL with new password |
| 2.7 | If `version_default_id > 0`: set `core_version_id = data.velodb_warehouse_versions.v.default_id` | `POST /settings/upgrade` with `targetVersionId:int64`, status `Running → Upgrading → Running` | apply succeeds, post-plan clean |
| 2.8 | If `version_default_id == 0`: set `core_version_id = data.velodb_warehouse_versions.v.default_id` | provider blocks with new guard error | apply fails fast with "Invalid core_version_id" — never hits API |

## Phase 3 — Cluster regressions (mixed billing, actions) (~25 min)

Validates that runtime-only features the new spec dropped *still work* through the provider.

```hcl
resource "velodb_cluster" "mix" {
  warehouse_id = velodb_warehouse.t.id
  name         = "mix"
  cluster_type = "COMPUTE"
  zone         = "us-east-1d"
  subscription_node_count = 1

  subscription { compute_vcpu = 4; cache_gb = 100; period = 1; period_unit = "Month"; auto_renew = false }
  on_demand    { compute_vcpu = 4; cache_gb = 100 }
}
```

| # | Test | Pass criterion |
|---|---|---|
| 3.1 | `terraform apply` to create mixed cluster | Cluster reaches `Running`, both `billingPools.subscription` and `billingPools.onDemand` populated in `GET /clusters/{id}` |
| 3.2 | Set `desired_state = "paused"` | `POST /clusters/{id}/pause` (not `/actions`), status `Running → Stopped` |
| 3.3 | Set `desired_state = "running"` | `POST /clusters/{id}/resume`, status `Stopped → Running` |
| 3.4 | Set `desired_state = "rebooting"` (or equivalent) | `POST /clusters/{id}/reboot` |
| 3.5 | `terraform plan` after each action | clean (no spurious billing-field drift) |

## Phase 4 — Resize, mixed billing focus (~30 min)

The most fragile path. Each step is a single `apply` against the cluster from Phase 3.

| # | Mutation | Expected | Risk |
|---|---|---|---|
| 4.1 | `on_demand.compute_vcpu = 4 → 8`, leave `cache_gb` unset | `cacheGbAutoScaleOnVcpuChange` marks cache_gb unknown; single PATCH for vcpu; status `Resizing → Running` | auto-scale leaks into subscription pool's cache_gb |
| 4.2 | `on_demand.cache_gb = 100 → 200` only | single disk-only PATCH, no proportional auto-scale | "no cluster changes to update" 409 if state not flushed |
| 4.3 | `subscription.compute_vcpu = 4 → 8` only | PATCH against subscription pool, on_demand unchanged in API response | subscription mid-term resize may 409 |
| 4.4 | Bump vcpu on **both** pools in same apply | sequential PATCHes (subscription first per current code), no race | order bug, partial-success leaving inconsistent state |
| 4.5 | Combined CPU+disk in same apply on subscription pool: vcpu 8→16 AND explicit cache_gb 200→1600 | provider splits into 2 sequential PATCHes (CPU first, disk second per spec rule) | "Do not combine CPU expansion and disk expansion" 400 if not split |
| 4.6 | `subscription_node_count = 1 → 2` | PATCH succeeds mid-term, billing reflects new count | API may reject node-count decrease |
| 4.7 | Drop `on_demand{}` block while subscription pool changes | state machine handles "remove on_demand" + resize-subscription in correct order | removing a pool with live nodes 409 |
| 4.8 | After every 4.x step: `terraform plan` | **must show no diff** | cache_gb auto-scale modifier mis-predicting API rounding |

Cleanup note: prepaid clusters can't be deleted — the cluster from this phase will outlive the test until expiry (~1 month).

## Phase 5 — Edge cases & negative tests (~15 min)

| # | Scenario | Pass criterion |
|---|---|---|
| 5.1 | `core_version_id = 999999999` (invalid) | API returns 409 "targetVersionId not found"; provider surfaces it cleanly |
| 5.2 | `start_hour_utc = -1` | validator rejects in `plan` |
| 5.3 | `upgrade_policy = ""` (empty string) | API behaviour TBD — capture and document |
| 5.4 | Resize while cluster is `Stopped` | Document expected behaviour: API may 409 "cluster not running" |
| 5.5 | List warehouses with org that has 21+ warehouses (if available) | data source returns all pages — current code hardcodes `size=20`; **expect bug** |
| 5.6 | API rate-limit (50 reads/min) — burst plan with many data sources | transport retries with backoff; no spurious failures |

## Phase 6 — Import flow (~15 min)

| # | Test | Pass criterion |
|---|---|---|
| 6.1 | `terraform import velodb_warehouse.imp AWVA7G1I` | state populated; `terraform plan` shows only fields not readable from API (e.g. admin_password) |
| 6.2 | Import a cluster: `terraform import velodb_cluster.imp WH/CL` | state populated; **no spurious diff on `subscription{}`/`on_demand{}` blocks** if billing pools match |
| 6.3 | Import warehouse with maintenance_window already set | state.maintenance_window matches API response shape |

## Phase 7 — Stale-config canary (~5 min)

Goal: confirm v0.x configurations fail loudly, not silently.

| # | Stale field in config | Pass criterion |
|---|---|---|
| 7.1 | `maintainability_start_time = "02:00"` | `terraform validate` fails: "Unsupported argument" |
| 7.2 | `maintainability_end_time = "06:00"` | same |
| 7.3 | `advanced_settings = jsonencode({...})` | same |
| 7.4 | `core_version = "3.1.0"` (string, settable) | `terraform plan` fails: "core_version is read-only" or shows perpetual drift |

## Phase 8 — Destroy (~5 min)

| # | Test | Pass criterion |
|---|---|---|
| 8.1 | `terraform destroy` Phase 1 warehouse | warehouse deleted, status reaches `Deleted` or 404 |
| 8.2 | If destroy fails with "Prepaid cluster cannot be deleted" | Document and accept — known API constraint |

## Phase 9 — Deferred / not in scope

These are intentionally **not** tested in this round:

- BYOC advanced mode (`/v1/cloud-settings/{cp}/credentials`, `/network-configs`) — provider does not expose these resources yet
- Unified `/v1/warehouses/{id}/connections` data source migration — current code uses legacy split endpoints which still work runtime-side
- New `/v1/private-link/warehouses/{id}/endpoints` POST/DELETE endpoint — sandbox returns 404 for it; not yet deployed
- State migration from v0.x state files (`StateUpgrader`) — no users on v0.x
- Cross-cloud (Azure, GCP, Aliyun) — only AWS sandbox available

## Sign-off criteria

Migration is releasable when:

- [ ] Phase 0 + Phase 1 + Phase 2 (excluding 2.7 if no versions available) green
- [ ] Phase 3 green — proves cluster regressions clean
- [ ] Phase 4.1 + 4.2 + 4.3 + 4.8 green — minimum resize coverage
- [ ] Phase 7 green — stale configs fail clearly
- [ ] Phase 5.1 + 5.2 green — invalid inputs caught at the right layer
- [ ] At least one mixed-billing import (Phase 6.2) showed no spurious diff

Phases 4.4–4.7, 5.3–5.6, and 6.3 are nice-to-have; document failures, don't block release.
