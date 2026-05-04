#!/usr/bin/env bash
# Phase 4 — on_demand cluster resize with drift assertions.
# Assumes Phase 3 left the cluster running with vcpu=4, cache_gb=100.
# Skips Phase 4b (mixed-billing & resize alongside prepaid) — that path
# requires preexisting prepaid clusters in the warehouse, which the API
# refuses to provision via PATCH in the current sandbox. Re-enable when
# the sandbox supports it.
set -euo pipefail
cd "$(dirname "$0")/../phase1"

: "${TF_VAR_api_key:?TF_VAR_api_key required}"

assert_clean() {
  local label="$1"
  shift
  set +e
  terraform plan -detailed-exitcode -no-color "$@" > /tmp/p4.log 2>&1
  rc=$?
  set -e
  if [ "$rc" -ne 0 ]; then
    echo "FAIL: drift after $label"
    cat /tmp/p4.log
    exit 1
  fi
  echo "OK: $label drift clean"
}

# 4.1 — vcpu 4→8, cache_gb omitted (let API auto-scale)
echo "=== 4.1: vcpu 4→8 with auto-scaled cache ==="
terraform apply -auto-approve -no-color \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" \
  -var="cluster_cache_gb_set=false"
assert_clean "4.1" \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" \
  -var="cluster_cache_gb_set=false"

# 4.2 — explicit cache_gb resize without vcpu change
# (the cluster's auto-scaled cache is now ~200, ramp it explicitly to 400)
echo "=== 4.2: explicit cache_gb resize, vcpu unchanged ==="
terraform apply -auto-approve -no-color \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=400"
assert_clean "4.2" \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=400"

echo
echo "=== Phase 4 complete: on_demand resize works without drift ==="
