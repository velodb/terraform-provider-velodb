data "velodb_aws_data_access_policy" "data" {
  bucket_name = aws_s3_bucket.data.bucket
  role_arn    = aws_iam_role.data.arn
}
