data "velodb_aws_crossaccount_policy" "deployment" {
  bucket_name         = aws_s3_bucket.data.bucket
  data_credential_arn = aws_iam_instance_profile.data.arn
}
