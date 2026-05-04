#!/usr/bin/env bash
# Phase 3 — cluster regression: pause / resume / reboot via desired_state +
# reboot_trigger on a pure on_demand cluster (Phases 0-2 should leave the
# warehouse in a known-good state).
set -euo pipefail
cd "$(dirname "$0")/../phase1"

: "${TF_VAR_api_key:?TF_VAR_api_key required}"

assert_clean() {
  set +e
  terraform plan -detailed-exitcode -no-color "$@" > /tmp/p3.log 2>&1
  rc=$?
  set -e
  if [ "$rc" -ne 0 ]; then
    echo "FAIL: drift after $1"
    cat /tmp/p3.log
    exit 1
  fi
}

# 3.1 — create on_demand cluster
echo "=== 3.1: create on_demand cluster ==="
terraform apply -auto-approve -no-color -var="include_cluster=true"
assert_clean "3.1" -var="include_cluster=true"

# 3.2 — pause
echo "=== 3.2: desired_state=paused ==="
terraform apply -auto-approve -no-color -var="include_cluster=true" -var="cluster_desired_state=paused"

# 3.3 — resume
echo "=== 3.3: desired_state=running ==="
terraform apply -auto-approve -no-color -var="include_cluster=true" -var="cluster_desired_state=running"

# 3.4 — reboot via reboot_trigger increment
echo "=== 3.4: reboot_trigger=1 ==="
terraform apply -auto-approve -no-color -var="include_cluster=true" -var="cluster_reboot_trigger=1"

# 3.5 — final drift
echo "=== 3.5: drift check ==="
assert_clean "3.5" -var="include_cluster=true" -var="cluster_reboot_trigger=1"

echo
echo "=== Phase 3 complete: pause/resume/reboot all on the new explicit endpoints ==="
