# Regression test for the zone-replacement subnet deadlock.
#
# Before the fix, aws_subnet.private derived its CIDR from the position of the AZ
# in var.zones (index(var.zones, each.key) + 1). Swapping us-east-1b for
# us-east-1c handed the new us-east-1c subnet the CIDR that us-east-1b still
# occupied, so the new subnet could not be created while the old one could not be
# deleted (its ENI was held by the VeloDB VPC endpoint), deadlocking the apply.
#
# The fix keys the CIDR on the AZ letter, so every AZ always maps to the same
# block regardless of list position. These plan-only tests assert that:
#   1. Each AZ maps to a stable CIDR across two different zone lists.
#   2. The swapped-in zone never inherits the CIDR of the swapped-out zone.
# Providers are mocked so the test needs no AWS or VeloDB credentials.

mock_provider "aws" {}

mock_provider "velodb" {}

variables {
  region         = "us-east-1"
  bucket_name    = "velodb-byoc-zone-test"
  name_prefix    = "zone-test"
  admin_password = "test-password-123"
  vpc_cidr       = "10.58.0.0/16"
}

override_data {
  target = data.velodb_byoc_prerequisites.aws
  values = {
    zones                       = [for z in ["us-east-1a", "us-east-1b", "us-east-1c", "us-east-1d"] : { zone = z }]
    endpoint_service_name       = "com.amazonaws.vpce.us-east-1.vpce-svc-test"
    deployment_assumer_role_arn = "arn:aws:iam::123456789012:role/assumer"
    external_id                 = "test-external-id"
  }
}

override_data {
  target = data.velodb_aws_crossaccount_policy.deployment
  values = {
    json = jsonencode({ Version = "2012-10-17", Statement = [for i in range(17) : { Effect = "Allow", Action = "*", Resource = "*" }] })
  }
}

override_data {
  target = data.velodb_aws_data_access_policy.data_access
  values = {
    json = jsonencode({ Version = "2012-10-17", Statement = [for i in range(3) : { Effect = "Allow", Action = "*", Resource = "*" }] })
  }
}

run "baseline_zones_abd" {
  command = plan

  variables {
    zones = ["us-east-1a", "us-east-1b", "us-east-1d"]
  }

  assert {
    condition     = aws_subnet.private["us-east-1a"].cidr_block == "10.58.16.0/20"
    error_message = "us-east-1a must map to the block for AZ letter 'a'."
  }

  assert {
    condition     = aws_subnet.private["us-east-1b"].cidr_block == "10.58.32.0/20"
    error_message = "us-east-1b must map to the block for AZ letter 'b'."
  }

  assert {
    condition     = aws_subnet.private["us-east-1d"].cidr_block == "10.58.64.0/20"
    error_message = "us-east-1d must map to the block for AZ letter 'd', independent of its list position."
  }
}

run "swapped_zones_acd" {
  command = plan

  variables {
    zones = ["us-east-1a", "us-east-1c", "us-east-1d"]
  }

  # us-east-1a and us-east-1d keep the exact CIDRs they had in the baseline run:
  # replacing us-east-1b with us-east-1c does not disturb the surviving subnets.
  assert {
    condition     = aws_subnet.private["us-east-1a"].cidr_block == "10.58.16.0/20"
    error_message = "us-east-1a CIDR must stay stable when another zone is swapped."
  }

  assert {
    condition     = aws_subnet.private["us-east-1d"].cidr_block == "10.58.64.0/20"
    error_message = "us-east-1d CIDR must stay stable when another zone is swapped."
  }

  # The core regression: us-east-1c gets its own block, NOT the 10.58.32.0/20
  # block that us-east-1b occupied. No CIDR conflict, so no replacement deadlock.
  assert {
    condition     = aws_subnet.private["us-east-1c"].cidr_block == "10.58.48.0/20"
    error_message = "us-east-1c must get the block for AZ letter 'c'."
  }

  assert {
    condition     = aws_subnet.private["us-east-1c"].cidr_block != "10.58.32.0/20"
    error_message = "us-east-1c must not inherit the CIDR that us-east-1b used, which would deadlock subnet replacement."
  }
}

# An explicit subnet_cidrs override wins for the named zone (e.g. to pin an
# existing deployment's CIDR before upgrading), while unset zones keep their
# derived block. This is what lets a gapped-zone deployment upgrade in place
# without a subnet/network/warehouse replacement.
run "explicit_cidr_override" {
  command = plan

  variables {
    zones = ["us-east-1a", "us-east-1b", "us-east-1d"]
    subnet_cidrs = {
      "us-east-1d" = "10.58.48.0/20"
    }
  }

  assert {
    condition     = aws_subnet.private["us-east-1d"].cidr_block == "10.58.48.0/20"
    error_message = "us-east-1d must use the explicit subnet_cidrs override, not the derived block."
  }

  assert {
    condition     = aws_subnet.private["us-east-1a"].cidr_block == "10.58.16.0/20"
    error_message = "Zones without an override must keep their derived block."
  }

  assert {
    condition     = aws_subnet.private["us-east-1b"].cidr_block == "10.58.32.0/20"
    error_message = "Zones without an override must keep their derived block."
  }
}
