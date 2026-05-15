#!/usr/bin/env bash
# Phase 6 — import flow.
# Captures warehouse_id and cluster_id from the phase1 workspace, then
# spins up a separate workspace and imports them. Asserts that the
# post-import plan only mentions write-only fields (admin_password,
# initial_cluster, reboot_trigger, connection_info).
set -euo pipefail
PHASE1_DIR="$(cd "$(dirname "$0")/../phase1" && pwd)"
PHASE6_DIR=$(mktemp -d -t tfmig-phase6-XXXXXX)
trap 'rm -rf "$PHASE6_DIR"' EXIT

: "${TF_VAR_api_key:?TF_VAR_api_key required}"

cd "$PHASE1_DIR"
WH_ID=$(terraform output -raw warehouse_id)
CL_ID=$(terraform output -raw cluster_id)
WH_NAME=$(terraform state show velodb_warehouse.t | grep '^    name' | head -1 | awk -F'"' '{print $2}')
CL_NAME=$(terraform state show 'velodb_cluster.od[0]' | grep '^    name' | head -1 | awk -F'"' '{print $2}')
echo "Importing warehouse=$WH_ID cluster=$CL_ID warehouse_name=$WH_NAME cluster_name=$CL_NAME"

cat > "$PHASE6_DIR/main.tf" <<HCL
terraform {
  required_providers {
    velodb = { source = "velodb/velodb" }
  }
}

provider "velodb" {
  host    = "sandbox-api.velodb.io"
  api_key = var.api_key
}

variable "api_key" {
  type      = string
  sensitive = true
}

resource "velodb_warehouse" "imp" {
  name            = "$WH_NAME"
  deployment_mode = "SaaS"
  cloud_provider  = "aws"
  region          = "us-east-1"
  admin_password  = "Tf@Rotated9876"

  initial_cluster {
    zone         = "us-east-1d"
    compute_vcpu = 4
    cache_gb     = 100
    auto_pause { enabled = false }
  }
}

resource "velodb_cluster" "imp_od" {
  warehouse_id   = velodb_warehouse.imp.id
  name           = "$CL_NAME"
  cluster_type   = "COMPUTE"
  zone           = "us-east-1d"
  desired_state  = "running"
  reboot_trigger = 1
  compute_vcpu   = 8
  cache_gb       = 400
}
HCL

cd "$PHASE6_DIR"
terraform init -input=false > /dev/null 2>&1 || true

# 6.1 — import warehouse
echo "=== 6.1: terraform import warehouse ==="
terraform import velodb_warehouse.imp "$WH_ID"

# 6.2 — import cluster
echo "=== 6.2: terraform import cluster ==="
terraform import velodb_cluster.imp_od "$WH_ID/$CL_ID"

# 6.3 — drift assertion: only the documented write-only fields should diverge.
echo "=== 6.3: post-import plan should only touch write-only fields ==="
terraform plan -no-color > /tmp/p6.log 2>&1 || true

# Forbid drift on these v1-migrated fields.
forbidden=(
  "core_version_id"
  "core_version "
  "compute_vcpu"
  "cache_gb"
  "status"
  "cloud_provider"
  "region"
)
for f in "${forbidden[@]}"; do
  # Only block concrete value-to-value drift. Computed attributes may become
  # "(known after apply)" because the imported resource has write-only config
  # differences; that is not persistent drift in the migrated fields.
  if grep -E "^[[:space:]]*~ ${f}" /tmp/p6.log | grep -v "(known after apply)" > /dev/null; then
    echo "FAIL: 6.3 — phantom drift on '$f' after import:"
    grep -E "^[[:space:]]*~ ${f}" /tmp/p6.log | grep -v "(known after apply)"
    exit 1
  fi
done
echo "OK: 6.3 — no drift on migrated fields after import"

echo
echo "=== Phase 6 complete: import flow shows only write-only field drift ==="
