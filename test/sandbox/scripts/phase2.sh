#!/usr/bin/env bash
# Phase 2 — warehouse mutations.
# Drives a sequence of -var overrides on the same workspace from phase1/.
# Aborts at first failure.
set -euo pipefail
cd "$(dirname "$0")/../phase1"

: "${TF_VAR_api_key:?TF_VAR_api_key required}"

# Helper: assert no drift.
assert_clean() {
  set +e
  terraform plan -detailed-exitcode -no-color > /tmp/p2.log 2>&1
  rc=$?
  set -e
  if [ "$rc" -eq 2 ]; then
    echo "FAIL: drift after $1"
    cat /tmp/p2.log
    exit 1
  fi
  if [ "$rc" -ne 0 ]; then
    echo "FAIL: plan errored after $1"
    cat /tmp/p2.log
    exit 1
  fi
  echo "OK: clean plan after $1"
}

# 2.1 — change start hour
echo "=== Phase 2.1: change maintenance_window.start_hour_utc ==="
terraform apply -auto-approve -no-color -var="maintenance_start_hour=6" -var="maintenance_end_hour=7"
assert_clean "2.1"

# 2.4 — invalid hour value (must fail at validation, never API)
echo "=== Phase 2.4: validator rejects start_hour_utc=25 ==="
set +e
terraform plan -no-color -var="maintenance_start_hour=25" 2>&1 | tee /tmp/p24.log
rc=${PIPESTATUS[0]}
set -e
if [ "$rc" -eq 0 ]; then
  echo "FAIL: validator did not reject start_hour_utc=25"
  exit 1
fi
grep -q "must be between 0 and 23" /tmp/p24.log || {
  echo "FAIL: expected 'must be between 0 and 23' message"
  exit 1
}
echo "OK: validator rejected"

# 2.5 — rename
echo "=== Phase 2.5: rename ==="
terraform apply -auto-approve -no-color -var="warehouse_name_override=tfmig-ci-renamed"
assert_clean "2.5"

# Restore name
terraform apply -auto-approve -no-color -var="warehouse_name_override="

echo "All Phase 2 mutations passed."
