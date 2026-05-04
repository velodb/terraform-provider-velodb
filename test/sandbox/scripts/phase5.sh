#!/usr/bin/env bash
# Phase 5 — edge cases and validators.
# Most run as terraform plan only (no API calls) so they're cheap and safe.
# 5.4 (resize while stopped) does require a real cluster, so it operates
# on the existing one from Phase 4.
set -euo pipefail
cd "$(dirname "$0")/../phase1"

: "${TF_VAR_api_key:?TF_VAR_api_key required}"

assert_validator_rejects() {
  local label="$1"
  local needle="$2"
  shift 2
  set +e
  terraform plan -no-color "$@" > /tmp/p5.log 2>&1
  rc=$?
  set -e
  if [ "$rc" -eq 0 ]; then
    echo "FAIL: $label — validator did not reject"
    cat /tmp/p5.log
    exit 1
  fi
  if ! grep -qF "$needle" /tmp/p5.log; then
    echo "FAIL: $label — error message missing '$needle'"
    cat /tmp/p5.log
    exit 1
  fi
  echo "OK: $label — '$needle'"
}

# 5.1 — invalid core_version_id (large fake) hits API 409
echo "=== 5.1: invalid core_version_id surfaces API 409 ==="
set +e
terraform apply -auto-approve -no-color \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=400" \
  -var="core_version_id=999999999" > /tmp/p5.log 2>&1
rc=$?
set -e
if [ "$rc" -eq 0 ]; then
  echo "FAIL: 5.1 — invalid core_version_id should have failed"
  exit 1
fi
if ! grep -q "targetVersionId not found\|InvalidParameter\|OperationConflict" /tmp/p5.log; then
  echo "FAIL: 5.1 — expected API error envelope"
  cat /tmp/p5.log
  exit 1
fi
echo "OK: 5.1 — surfaces API 409 cleanly"

# 5.2 — negative hour value rejected by validator (no API call)
echo "=== 5.2: negative hour rejected ==="
assert_validator_rejects "5.2" "must be between 0 and 23" \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="maintenance_start_hour=-1"

# 5.3 — empty upgrade_policy rejected by validator
echo "=== 5.3: empty upgrade_policy rejected ==="
assert_validator_rejects "5.3" "string length must be at least 1" \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="upgrade_policy="

# 5.4 — vcpu < 4 rejected by validator (cluster pool)
echo "=== 5.4: cluster vcpu < 4 rejected ==="
assert_validator_rejects "5.4" "must be at least 4" \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=2"

# 5.5 — cache_gb < 100 rejected
echo "=== 5.5: cluster cache_gb < 100 rejected ==="
assert_validator_rejects "5.5" "must be at least 100" \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=50"

# 5.6 — resize while paused returns API 409 cleanly
echo "=== 5.6: resize while paused returns API 409 ==="
# First pause the cluster
terraform apply -auto-approve -no-color \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=400" \
  -var="cluster_desired_state=paused"
# Try to resize while paused — should fail with API 409
set +e
terraform apply -auto-approve -no-color \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=16" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=400" \
  -var="cluster_desired_state=paused" > /tmp/p5.log 2>&1
rc=$?
set -e
if [ "$rc" -eq 0 ]; then
  echo "FAIL: 5.6 — resize-while-paused should 409"
  exit 1
fi
if ! grep -q "cluster is not running\|OperationConflict" /tmp/p5.log; then
  echo "FAIL: 5.6 — expected 'cluster is not running' 409"
  cat /tmp/p5.log
  exit 1
fi
echo "OK: 5.6 — resize-while-paused 409 surfaced cleanly"
# Restore
terraform apply -auto-approve -no-color \
  -var="include_cluster=true" \
  -var="cluster_reboot_trigger=1" \
  -var="cluster_vcpu=8" \
  -var="cluster_cache_gb_set=true" \
  -var="cluster_cache_gb=400" \
  -var="cluster_desired_state=running"

echo
echo "=== Phase 5 complete: 6 edge cases covered ==="
