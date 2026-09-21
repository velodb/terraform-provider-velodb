data "velodb_aws_assume_role_policy" "deployment" {
  principal_arn = data.velodb_byoc_prerequisites.aws.deployment_assumer_role_arn
  external_id   = data.velodb_byoc_prerequisites.aws.external_id
}
