mock_provider "aws" {}
mock_provider "velodb" {}

variables {
  region                    = "us-east-1"
  name_prefix               = "bucket-test"
  admin_password            = "test-password-123"
  vpc_id                    = "vpc-test"
  subnet_ids_by_zone        = { "us-east-1a" = "subnet-test" }
  security_group_id         = "sg-test"
  endpoint_id               = "vpce-test"
  data_credential_arn       = "arn:aws:iam::111122223333:instance-profile/data"
  deployment_credential_arn = "arn:aws:iam::111122223333:role/deploy"
}

run "leading_hyphen" {
  command = plan

  plan_options {
    target = [data.aws_s3_bucket.selected]
  }

  variables {
    bucket_name = "-invalid"
  }

  expect_failures = [var.bucket_name]
}

run "uppercase" {
  command = plan

  plan_options {
    target = [data.aws_s3_bucket.selected]
  }

  variables {
    bucket_name = "Invalid-bucket"
  }

  expect_failures = [var.bucket_name]
}

run "adjacent_periods" {
  command = plan

  plan_options {
    target = [data.aws_s3_bucket.selected]
  }

  variables {
    bucket_name = "invalid..bucket"
  }

  expect_failures = [var.bucket_name]
}

run "reserved_suffix" {
  command = plan

  plan_options {
    target = [data.aws_s3_bucket.selected]
  }

  variables {
    bucket_name = "bucket--x-s3"
  }

  expect_failures = [var.bucket_name]
}

run "valid_bucket" {
  command = plan

  plan_options {
    target = [data.aws_s3_bucket.selected]
  }

  variables {
    bucket_name = "velodb-data"
  }
}

run "valid_dotted_bucket" {
  command = plan

  plan_options {
    target = [data.aws_s3_bucket.selected]
  }

  variables {
    bucket_name = "velodb.data"
  }
}

run "account_regional_bucket" {
  command = plan

  plan_options {
    target = [data.aws_s3_bucket.selected]
  }

  variables {
    bucket_name = "data-111122223333-us-east-1-an"
  }
}
