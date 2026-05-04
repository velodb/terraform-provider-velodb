# Sandbox tests

Live tests against `https://sandbox.velodb.io`. **Provisions real warehouses and clusters — costs money.**

## Layout

```
test/sandbox/
├── phase1/          # base Terraform workspace (warehouse + versions data source)
├── scripts/
│   ├── phase2.sh    # warehouse mutations: maintenance window, validators, rename
│   └── phase3.sh    # cluster regressions: pause/resume/reboot
└── README.md
```

## Running locally

```bash
# Build + install provider
cd terraform-provider-velodb
go install .

# Configure dev_overrides (one-time)
cat > ~/.terraformrc <<EOF
provider_installation {
  dev_overrides {
    "velodb/velodb" = "$(go env GOPATH)/bin"
  }
  direct {}
}
EOF

# Set sandbox API key
export TF_VAR_api_key='sk-...'

# Run a phase
cd test/sandbox/phase1
terraform apply -auto-approve   # Phase 1
bash ../scripts/phase2.sh        # Phase 2
bash ../scripts/phase3.sh        # Phase 3

# Clean up
terraform destroy -auto-approve
```

## CI

`.github/workflows/sandbox-tests.yml` is `workflow_dispatch` only — never auto-triggers. Run from the GitHub Actions UI when you want a clean live run.

Required repo secret: `VELODB_SANDBOX_API_KEY`.

## Known sandbox quirks (not provider bugs)

- **Mixed billing PATCH** (`PATCH /clusters/{id}` with `billingModel=subscription` to add a subscription pool to an existing on_demand cluster) returns `409 OperationConflict — The requested parameters did not meet the requirements when scaling the cluster out or in` for all sizes tested. The `convert-to-subscription` endpoint works as a full conversion. Mixed-billing tests are skipped here until the sandbox supports the PATCH path again.
- **Prepaid clusters cannot be deleted** — once a cluster is on subscription billing, `DELETE` returns 409 until expiry. Avoid creating prepaid clusters in throwaway tests.
- **`/v1/warehouses/{id}/versions`** returns `data: []` for fresh warehouses — version-upgrade tests are skipped when the list is empty.
- **`/v1/private-link/warehouses/{id}/endpoints`** (new) returns 404 in sandbox even though it's documented; legacy `/connections/private/...` still works at runtime.
