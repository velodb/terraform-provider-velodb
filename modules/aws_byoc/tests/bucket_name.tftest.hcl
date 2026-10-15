mock_provider "aws" {}
mock_provider "velodb" {}

variables {
  region         = "us-east-1"
  name_prefix    = "bucket-test"
  admin_password = "test-password-123"
  zones          = ["us-east-1a", "us-east-1b", "us-east-1c"]
}

run "leading_hyphen" {
  command = plan

  plan_options {
    target = [aws_s3_bucket.data]
  }

  variables {
    bucket_name = "-invalid"
  }

  expect_failures = [var.bucket_name]
}

run "uppercase" {
  command = plan

  plan_options {
    target = [aws_s3_bucket.data]
  }

  variables {
    bucket_name = "Invalid-bucket"
  }

  expect_failures = [var.bucket_name]
}

run "adjacent_periods" {
  command = plan

  plan_options {
    target = [aws_s3_bucket.data]
  }

  variables {
    bucket_name = "invalid..bucket"
  }

  expect_failures = [var.bucket_name]
}

run "reserved_suffix" {
  command = plan

  plan_options {
    target = [aws_s3_bucket.data]
  }

  variables {
    bucket_name = "bucket--x-s3"
  }

  expect_failures = [var.bucket_name]
}

run "valid_bucket" {
  command = plan

  plan_options {
    target = [aws_s3_bucket.data]
  }

  variables {
    bucket_name = "velodb-data"
  }
}

run "valid_dotted_bucket" {
  command = plan

  plan_options {
    target = [aws_s3_bucket.data]
  }

  variables {
    bucket_name = "velodb.data"
  }
}

run "account_regional_bucket" {
  command = plan

  plan_options {
    target = [aws_s3_bucket.data]
  }

  variables {
    bucket_name = "data-111122223333-us-east-1-an"
  }

  expect_failures = [var.bucket_name]
}
