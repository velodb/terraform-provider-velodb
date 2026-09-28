# Immutable warehouse inputs

Run `python3 test/immutable_inputs/check.py` from any directory. Requires Terraform
on PATH. The script copies the actual guard resources from both BYOC modules into
temporary configurations and uses only Terraform's built-in provider. It creates
local state, checks unchanged plans, verifies each input edit fails planning,
checks both directions of the encryption flags, and verifies explicit destruction.
It never connects to AWS or VeloDB.

Provider integration coverage uses a local mock API:

```sh
TF_ACC=1 go test ./internal/provider -run 'TestWarehouse(InfrastructureEditsRejected|DependencyReplacementRejected)' -count=1 -v
```

These tests verify region, initial zone, encryption key additions, and indirect
bucket/network replacements fail planning while unchanged plans remain valid.
