data "velodb_aws_data_access_assume_role_policy" "data" {
  role_arn = local.data_role_arn
}
