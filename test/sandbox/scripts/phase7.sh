#!/usr/bin/env bash
# Phase 7 — stale-config canary.
# Verifies that every v0.x field that was removed in v1 produces a clear
# schema/validate error. No API calls.
set -euo pipefail
WORKDIR=$(mktemp -d -t tfmig-phase7-XXXXXX)
trap 'rm -rf "$WORKDIR"' EXIT
cd "$WORKDIR"

cat > base.tf <<'HCL'
terraform {
  required_providers {
    velodb = { source = "velodb/velodb" }
  }
}

provider "velodb" {
  host    = "sandbox.velodb.io"
  api_key = "x"
}
HCL

# Helper: write a stale-field config, run validate, expect a specific error.
expect_validate_error() {
  local label="$1"
  local needle="$2"
  local hcl="$3"
  cat > stale.tf <<HCL
resource "velodb_warehouse" "stale" {
  name            = "stale"
  deployment_mode = "SaaS"
  cloud_provider  = "aws"
  region          = "us-east-1"
  admin_password  = "x"

  $hcl

  initial_cluster {
    name         = "default"
    zone         = "us-east-1d"
    compute_vcpu = 4
    cache_gb     = 100
    auto_pause { enabled = false }
  }
}
HCL
  set +e
  terraform validate -no-color > /tmp/p7.log 2>&1
  rc=$?
  set -e
  if [ "$rc" -eq 0 ]; then
    echo "FAIL: $label — config validated when it should have failed"
    cat /tmp/p7.log
    exit 1
  fi
  if ! grep -qF "$needle" /tmp/p7.log; then
    echo "FAIL: $label — expected '$needle'"
    cat /tmp/p7.log
    exit 1
  fi
  echo "OK: $label — '$needle'"
}

terraform init -input=false > /dev/null 2>&1

# 7.1 — maintainability_start_time is gone
expect_validate_error "7.1 maintainability_start_time" \
  "An argument named \"maintainability_start_time\"" \
  'maintainability_start_time = "02:00"'

# 7.2 — maintainability_end_time is gone
expect_validate_error "7.2 maintainability_end_time" \
  "An argument named \"maintainability_end_time\"" \
  'maintainability_end_time = "06:00"'

# 7.3 — advanced_settings is gone
expect_validate_error "7.3 advanced_settings" \
  "An argument named \"advanced_settings\"" \
  'advanced_settings = jsonencode({ enableTde = 1 })'

# 7.4 — core_version is read-only now
expect_validate_error "7.4 core_version read-only" \
  "Invalid Configuration for Read-Only Attribute" \
  'core_version = "3.1.0"'

echo
echo "=== Phase 7 complete: all 4 stale fields fail loudly ==="
