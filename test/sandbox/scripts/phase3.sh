#!/usr/bin/env bash
# Phase 3 — cluster regression: pause / resume / reboot via desired_state + reboot_trigger.
# Adds a velodb_cluster.od to the existing phase1 workspace.
set -euo pipefail
cd "$(dirname "$0")/../phase1"

: "${TF_VAR_api_key:?TF_VAR_api_key required}"

# Append the cluster resource (idempotent — using a separate file).
cat > cluster.tf <<'HCL'
variable "cluster_desired_state" {
  type    = string
  default = "running"
}
variable "cluster_reboot_trigger" {
  type    = number
  default = 0
}

resource "velodb_cluster" "od" {
  warehouse_id   = velodb_warehouse.t.id
  name           = "od-test"
  cluster_type   = "COMPUTE"
  zone           = "us-east-1d"
  desired_state  = var.cluster_desired_state
  reboot_trigger = var.cluster_reboot_trigger

  on_demand {
    compute_vcpu = 4
    cache_gb     = 100
  }
}
HCL

# 3.1 create
echo "=== Phase 3.1: create on_demand cluster ==="
terraform apply -auto-approve -no-color
terraform plan -detailed-exitcode -no-color || { echo "FAIL: drift after create"; exit 1; }

# 3.2 pause
echo "=== Phase 3.2: pause via desired_state=paused ==="
terraform apply -auto-approve -no-color -var="cluster_desired_state=paused"
state_status=$(terraform output -raw warehouse_id) # warehouse running, but check cluster status from API isn't readily exposed.
echo "OK: pause apply succeeded"

# 3.3 resume
echo "=== Phase 3.3: resume ==="
terraform apply -auto-approve -no-color -var="cluster_desired_state=running"
echo "OK: resume apply succeeded"

# 3.4 reboot via reboot_trigger increment
echo "=== Phase 3.4: reboot ==="
terraform apply -auto-approve -no-color -var="cluster_reboot_trigger=1"
echo "OK: reboot apply succeeded"

# 3.5 final drift check
echo "=== Phase 3.5: drift check ==="
set +e
terraform plan -detailed-exitcode -no-color
rc=$?
set -e
if [ "$rc" -ne 0 ]; then
  echo "FAIL: drift after Phase 3 actions"
  exit 1
fi
echo "All Phase 3 cluster actions passed with no drift."
