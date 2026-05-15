# Sandbox tests

Live tests against `https://sandbox-api.velodb.io`. **Provisions real warehouses and clusters — costs money.**

## Layout

```
test/sandbox/
├── phase1/                      # base Terraform workspace (warehouse, optional cluster, versions data source)
├── scripts/
│   ├── phase2.sh                # warehouse mutations
│   ├── phase3.sh                # cluster regressions: pause/resume/reboot
│   ├── phase4.sh                # resize: vcpu + API-implied cache + explicit cache_gb
│   ├── phase5.sh                # edge cases: invalid IDs, validators, resize-while-paused
│   ├── phase6.sh                # import flow: warehouse + cluster, drift assertions
│   └── phase7.sh                # stale-config canary (no API calls)
└── README.md
```

The phases share state — phase2 → phase3 → phase4 → phase5 → phase6 each builds on the workspace produced by the previous phase. Run them in order.

## Phase coverage

| Phase | What it tests | API calls? |
|---|---|---|
| 0 | `go build`, `go vet`, `go test` | no |
| 1 | Fresh `apply` of new HCL shape, post-apply drift = 0 | yes (creates warehouse) |
| 2 | Rename, password rotation, invalid `core_version_id` | yes |
| 3 | Pause / resume / reboot via `desired_state` + `reboot_trigger`, post-apply drift = 0 | yes (creates cluster) |
| 4 | Resize: `vcpu 4→8` with API-implied cache, then explicit `cache_gb` resize | yes |
| 5 | Edge cases: invalid `core_version_id`, invalid `compute_vcpu`, `cache_gb < 100`, auto-pause timeout validation, resize-while-paused | partial — most are plan-only |
| 6 | Import warehouse + cluster into a fresh workspace; assert no drift on read-back fields | yes (read-only) |
| 7 | Confirm v0.x fields fail at `terraform validate` | no |

## Running locally

```bash
cd terraform-provider-velodb
go install .

cat > ~/.terraformrc <<EOF
provider_installation {
  dev_overrides {
    "velodb/velodb" = "$(go env GOPATH)/bin"
  }
  direct {}
}
EOF

export TF_VAR_api_key='sk-...'

# Run a phase
cd test/sandbox/phase1
terraform init -input=false
terraform apply -auto-approve            # Phase 1
bash ../scripts/phase2.sh                # Phase 2
bash ../scripts/phase3.sh                # Phase 3
bash ../scripts/phase4.sh                # Phase 4
bash ../scripts/phase5.sh                # Phase 5
bash ../scripts/phase6.sh                # Phase 6
bash ../scripts/phase7.sh                # Phase 7 — no API calls

# Clean up
terraform destroy -auto-approve \
  -var="include_cluster=true" -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" -var="cluster_cache_gb=400"
```

## CI

`.github/workflows/sandbox-tests.yml` is `workflow_dispatch` only — never auto-triggers. Run from the GitHub Actions UI when you want a clean live run. The default `phases` input is `0,1,2,3,4,5,6,7` — pass any comma-separated subset to run a smaller slice.

Required repo secret: `VELODB_SANDBOX_API_KEY`.

## Not in scope (sandbox quirks, not provider bugs)

- **Mixed-billing create/update** is not part of the current management API schema. Terraform exposes the observed `billing_model` as read-only.
- **Prepaid cluster delete lock** — once a cluster is on subscription billing, `DELETE` returns 409 until expiry. Avoid creating prepaid clusters in throwaway tests.
- **`/v1/warehouses/{id}/versions`** returns `data: []` for fresh warehouses, so Phase 2.7 (valid `core_version_id` upgrade) is documented but not asserted.
- **CPU resize cache floor** — increasing vCPU can automatically raise cache to the API-implied minimum. Phase 4 plans that minimum (`4/100 → 8/200`) before testing a separate cache-only resize.
- **PrivateLink endpoint deregistration** is not exposed by the current management API. Terraform can register endpoints, but destroy removes only local state.
