#!/usr/bin/env bash
# Phase 4 — cluster resize with drift assertions.
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

assert_apply_rejects() {
  local label="$1"
  local needle="$2"
  shift 2
  set +e
  terraform apply -auto-approve -no-color "$@" > /tmp/p4.log 2>&1
  rc=$?
  set -e
  if [ "$rc" -eq 0 ]; then
    echo "FAIL: $label — apply should have failed"
    cat /tmp/p4.log
    exit 1
  fi
  if ! grep -qF "$needle" /tmp/p4.log; then
    echo "FAIL: $label — error message missing '$needle'"
    cat /tmp/p4.log
    exit 1
  fi
  echo "OK: $label rejected with '$needle'"
}

# 4.1 — vcpu 4→8, cache_gb moves to the API-implied minimum
echo "=== 4.1: vcpu 4→8, cache_gb 100→200 ==="
terraform apply -auto-approve -no-color \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=200"
assert_clean "4.1" \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=200"

# 4.2 — explicit cache_gb resize without vcpu change
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

# 4.3 — CPU shrink without the API-implied cache_gb would drift; reject before mutation
echo "=== 4.3: reject vcpu 8→4 while keeping cache_gb=400 ==="
assert_apply_rejects "4.3" "compute_vcpu resize requires cache_gb update" \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=4" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=400"
assert_clean "4.3 state unchanged" \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=400"

# 4.4 — valid CPU shrink when cache_gb matches the API-implied value
echo "=== 4.4: valid vcpu 8→4, cache_gb 400→200 ==="
terraform apply -auto-approve -no-color \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=4" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=200"
assert_clean "4.4" \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=4" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=200"

# 4.5 — restore to the Phase 5 baseline; this also proves correct simultaneous CPU+cache is allowed
echo "=== 4.5: valid vcpu 4→8, cache_gb 200→400 ==="
terraform apply -auto-approve -no-color \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=400"
assert_clean "4.5" \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=400"

# 4.6 — cache shrink is not supported
echo "=== 4.6: reject cache_gb shrink 400→200 ==="
assert_apply_rejects "4.6" "cache_gb cannot be decreased" \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=200"

# 4.7 — arbitrary simultaneous CPU+cache changes must be split
echo "=== 4.7: reject non-implied simultaneous vcpu/cache_gb change ==="
assert_apply_rejects "4.7" "Simultaneous compute_vcpu and cache_gb changes are not supported" \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=16" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=600"

echo
echo "=== Phase 4 complete: resize works without drift ==="
