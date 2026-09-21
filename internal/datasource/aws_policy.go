package datasource

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const awsPolicyVersion = "2012-10-17"

var (
	awsRoleARNPattern            = regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/[A-Za-z0-9+=,.@_/-]+$`)
	awsInstanceProfileARNPattern = regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:instance-profile/[A-Za-z0-9+=,.@_/-]+$`)
	awsKMSKeyARNPattern          = regexp.MustCompile(`^arn:aws:kms:[a-z0-9-]+:[0-9]{12}:key/[A-Za-z0-9-]+$`)
)

type awsPolicyDocument struct {
	Version    string               `json:"Version"`
	Statements []awsPolicyStatement `json:"Statement"`
}

type awsPolicyStatement struct {
	Sid       string                       `json:"Sid,omitempty"`
	Effect    string                       `json:"Effect"`
	Actions   []string                     `json:"Action"`
	Resources []string                     `json:"Resource,omitempty"`
	Principal map[string]string            `json:"Principal,omitempty"`
	Condition map[string]map[string]string `json:"Condition,omitempty"`
}

func buildAWSAssumeRolePolicy(principalARN, externalID string) (string, error) {
	if !awsRoleARNPattern.MatchString(principalARN) {
		return "", fmt.Errorf("principal_arn must be a commercial AWS IAM role ARN")
	}
	if strings.TrimSpace(externalID) == "" {
		return "", fmt.Errorf("external_id must not be blank")
	}
	return marshalAWSPolicy([]awsPolicyStatement{{
		Effect:    "Allow",
		Actions:   []string{"sts:AssumeRole"},
		Principal: map[string]string{"AWS": principalARN},
		Condition: map[string]map[string]string{"StringEquals": {"sts:ExternalId": externalID}},
	}})
}

func buildAWSDataAccessAssumeRolePolicy(roleARN string) (string, error) {
	accountID, err := awsAccountIDFromRoleARN(roleARN)
	if err != nil {
		return "", err
	}
	return marshalAWSPolicy([]awsPolicyStatement{
		{
			Sid:       "EC2AssumeRole",
			Effect:    "Allow",
			Actions:   []string{"sts:AssumeRole"},
			Principal: map[string]string{"Service": "ec2.amazonaws.com"},
		},
		{
			Sid:       "SelfAssumeRole",
			Effect:    "Allow",
			Actions:   []string{"sts:AssumeRole"},
			Principal: map[string]string{"AWS": "arn:aws:iam::" + accountID + ":root"},
			Condition: map[string]map[string]string{"StringEquals": {"aws:PrincipalArn": roleARN}},
		},
	})
}

func buildAWSDataAccessPolicy(bucketName, roleARN, tdeKMSKeyARN string) (string, error) {
	if err := validateAWSBucketName(bucketName); err != nil {
		return "", err
	}
	if _, err := awsAccountIDFromRoleARN(roleARN); err != nil {
		return "", err
	}
	if tdeKMSKeyARN != "" && !awsKMSKeyARNPattern.MatchString(tdeKMSKeyARN) {
		return "", fmt.Errorf("tde_kms_arn must be a commercial AWS KMS key ARN")
	}

	bucketARN := "arn:aws:s3:::" + bucketName
	statements := []awsPolicyStatement{
		{
			Effect:    "Allow",
			Resources: []string{bucketARN},
			Actions: []string{
				"s3:GetBucketLocation", "s3:GetBucketVersioning", "s3:PutBucketCORS",
				"s3:ListBucket", "s3:ListBucketVersions", "s3:ListBucketMultipartUploads",
			},
		},
		{
			Effect:    "Allow",
			Resources: []string{bucketARN + "/*"},
			Actions: []string{
				"s3:GetObject", "s3:GetObjectVersion", "s3:PutObject", "s3:DeleteObject",
				"s3:DeleteObjectVersion", "s3:AbortMultipartUpload", "s3:ListMultipartUploadParts",
			},
		},
		{Effect: "Allow", Actions: []string{"sts:AssumeRole"}, Resources: []string{roleARN}},
	}
	if tdeKMSKeyARN != "" {
		statements = append(statements, awsPolicyStatement{
			Sid:       "KMSAccess",
			Effect:    "Allow",
			Actions:   []string{"kms:Encrypt", "kms:Decrypt", "kms:GenerateDataKey*", "kms:DescribeKey"},
			Resources: []string{tdeKMSKeyARN},
		})
	}
	return marshalAWSPolicy(statements)
}

func buildAWSCrossAccountPolicy(bucketName, instanceProfileARN string) (string, error) {
	if err := validateAWSBucketName(bucketName); err != nil {
		return "", err
	}
	roleARN, err := awsRoleARNFromInstanceProfileARN(instanceProfileARN)
	if err != nil {
		return "", err
	}
	bucketARN := "arn:aws:s3:::" + bucketName
	return marshalAWSPolicy([]awsPolicyStatement{
		{
			Effect: "Allow",
			Actions: []string{
				"ec2:TerminateInstances", "ec2:StopInstances", "ec2:StartInstances",
				"ec2:RebootInstances", "ec2:ModifyInstanceAttribute", "ec2:ModifyVolume",
			},
			Resources: []string{"arn:aws:ec2:*:*:volume/*", "arn:aws:ec2:*:*:instance/*"},
			Condition: map[string]map[string]string{"StringEquals": {"aws:ResourceTag/resource-created-by": "velodb"}},
		},
		{
			Effect: "Allow",
			Actions: []string{
				"ec2:DescribeVpcs", "ec2:DescribeSubnets", "ec2:DescribeAccountAttributes",
				"ec2:DescribeAddresses", "ec2:DescribeNatGateways", "ec2:DescribeInternetGateways",
				"ec2:DescribeInstances", "ec2:DescribeSecurityGroups", "ec2:DescribeSecurityGroupRules",
				"ec2:DescribeAvailabilityZones", "ec2:DescribeInstanceTypes", "ec2:ModifyInstanceAttribute",
				"ec2:DescribeVolumes", "ec2:DescribeImages", "ec2:DescribeVpcEndpoints",
				"ec2:DescribePrefixLists", "ec2:DescribeRouteTables", "ec2:DescribeTags",
				"elasticloadbalancing:DescribeLoadBalancers", "elasticloadbalancing:DescribeListeners",
				"elasticloadbalancing:DescribeLoadBalancerAttributes", "elasticloadbalancing:DescribeTargetGroupAttributes",
				"elasticloadbalancing:DescribeTags", "elasticloadbalancing:DescribeTargetHealth",
				"elasticloadbalancing:DescribeTargetGroups", "iam:GetPolicy", "iam:GetPolicyVersion",
			},
			Resources: []string{"*"},
		},
		{
			Effect:    "Allow",
			Actions:   []string{"ec2:RunInstances", "ec2:CreateTags"},
			Resources: []string{"arn:aws:ec2:*:*:volume/*", "arn:aws:ec2:*:*:instance/*", "arn:aws:ec2:*:*:network-interface/*"},
			Condition: map[string]map[string]string{"StringEquals": {"aws:RequestTag/resource-created-by": "velodb"}},
		},
		{
			Effect:    "Allow",
			Actions:   []string{"ec2:RunInstances"},
			Resources: []string{"arn:aws:ec2:*:*:image/*", "arn:aws:ec2:*:*:security-group/*", "arn:aws:ec2:*:*:subnet/*"},
		},
		{
			Effect:    "Allow",
			Actions:   []string{"ec2:CreateTags", "ec2:DeleteTags"},
			Resources: []string{"arn:aws:ec2:*:*:instance/*", "arn:aws:ec2:*:*:volume/*", "arn:aws:ec2:*:*:network-interface/*"},
			Condition: map[string]map[string]string{"StringEquals": {"ec2:ResourceTag/resource-created-by": "velodb"}},
		},
		{
			Effect:  "Allow",
			Actions: []string{"ec2:CreateVpcEndpoint"},
			Resources: []string{
				"arn:aws:ec2:*:*:vpc/*", "arn:aws:ec2:*:*:subnet/*", "arn:aws:ec2:*:*:route-table/*",
				"arn:aws:ec2:*:*:security-group/*", "arn:aws:ec2:*:*:vpc-endpoint-service/*",
			},
		},
		{
			Effect:    "Allow",
			Actions:   []string{"ec2:CreateVpcEndpoint"},
			Resources: []string{"arn:aws:ec2:*:*:vpc-endpoint/*"},
			Condition: map[string]map[string]string{"StringEquals": {"aws:RequestTag/resource-created-by": "velodb"}},
		},
		{
			Effect:    "Allow",
			Actions:   []string{"ec2:CreateTags"},
			Resources: []string{"arn:aws:ec2:*:*:vpc-endpoint/*"},
			Condition: map[string]map[string]string{"StringEquals": {"ec2:CreateAction": "CreateVpcEndpoint"}},
		},
		{
			Effect:    "Allow",
			Actions:   []string{"ec2:DeleteVpcEndpoints"},
			Resources: []string{"arn:aws:ec2:*:*:vpc-endpoint/*"},
			Condition: map[string]map[string]string{"StringEquals": {"ec2:ResourceTag/resource-created-by": "velodb"}},
		},
		{
			Effect:    "Allow",
			Actions:   []string{"elasticloadbalancing:CreateListener", "elasticloadbalancing:CreateLoadBalancer", "elasticloadbalancing:CreateTargetGroup"},
			Resources: []string{"arn:aws:elasticloadbalancing:*:*:targetgroup/*", "arn:aws:elasticloadbalancing:*:*:loadbalancer/*", "arn:aws:elasticloadbalancing:*:*:listener/*"},
			Condition: map[string]map[string]string{"StringEquals": {"aws:RequestTag/resource-created-by": "velodb"}},
		},
		{
			Effect: "Allow",
			Actions: []string{
				"elasticloadbalancing:RegisterTargets", "elasticloadbalancing:DeleteLoadBalancer",
				"elasticloadbalancing:ModifyTargetGroupAttributes", "elasticloadbalancing:DeregisterTargets",
				"elasticloadbalancing:DeleteTargetGroup", "elasticloadbalancing:ModifyLoadBalancerAttributes",
				"elasticloadbalancing:DeleteListener",
			},
			Resources: []string{"arn:aws:elasticloadbalancing:*:*:targetgroup/*", "arn:aws:elasticloadbalancing:*:*:loadbalancer/*", "arn:aws:elasticloadbalancing:*:*:listener/*"},
			Condition: map[string]map[string]string{"StringEquals": {"elasticloadbalancing:ResourceTag/resource-created-by": "velodb"}},
		},
		{
			Effect:    "Allow",
			Actions:   []string{"elasticloadbalancing:AddTags", "elasticloadbalancing:RemoveTags"},
			Resources: []string{"arn:aws:elasticloadbalancing:*:*:*"},
			Condition: map[string]map[string]string{"StringEquals": {"elasticloadbalancing:ResourceTag/resource-created-by": "velodb"}},
		},
		{
			Effect: "Allow",
			Actions: []string{
				"s3:GetBucketLocation", "s3:GetBucketVersioning", "s3:GetBucketPublicAccessBlock",
				"s3:GetLifecycleConfiguration", "s3:PutLifecycleConfiguration", "s3:ListBucket",
			},
			Resources: []string{bucketARN},
		},
		{
			Effect: "Allow",
			Actions: []string{
				"iam:GetInstanceProfile", "iam:GetRole", "iam:GetRolePolicy",
				"iam:ListRolePolicies", "iam:ListAttachedRolePolicies",
			},
			Resources: []string{instanceProfileARN, roleARN},
		},
		{
			Effect:    "Allow",
			Actions:   []string{"iam:PassRole"},
			Resources: []string{roleARN},
			Condition: map[string]map[string]string{"StringEquals": {"iam:PassedToService": "ec2.amazonaws.com"}},
		},
		{
			Effect:    "Allow",
			Actions:   []string{"iam:CreateServiceLinkedRole"},
			Resources: []string{"arn:aws:iam::*:role/aws-service-role/elasticloadbalancing.amazonaws.com/AWSServiceRoleForElasticLoadBalancing"},
			Condition: map[string]map[string]string{"StringEquals": {"iam:AWSServiceName": "elasticloadbalancing.amazonaws.com"}},
		},
		{
			Effect: "Allow",
			Actions: []string{
				"kms:DescribeKey", "kms:GetKeyPolicy", "kms:ListKeyPolicies", "kms:Encrypt",
				"kms:Decrypt", "kms:ReEncrypt*", "kms:GenerateDataKey*", "kms:CreateGrant",
			},
			Resources: []string{"*"},
		},
	})
}

func marshalAWSPolicy(statements []awsPolicyStatement) (string, error) {
	policy, err := json.MarshalIndent(awsPolicyDocument{Version: awsPolicyVersion, Statements: statements}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal AWS policy: %w", err)
	}
	return string(policy), nil
}

func awsAccountIDFromRoleARN(roleARN string) (string, error) {
	if !awsRoleARNPattern.MatchString(roleARN) {
		return "", fmt.Errorf("role_arn must be a commercial AWS IAM role ARN")
	}
	return strings.Split(roleARN, ":")[4], nil
}

func awsRoleARNFromInstanceProfileARN(instanceProfileARN string) (string, error) {
	if !awsInstanceProfileARNPattern.MatchString(instanceProfileARN) {
		return "", fmt.Errorf("data_credential_arn must be a commercial AWS IAM instance-profile ARN")
	}
	return strings.Replace(instanceProfileARN, ":instance-profile/", ":role/", 1), nil
}

func validateAWSBucketName(bucketName string) error {
	if strings.TrimSpace(bucketName) == "" {
		return fmt.Errorf("bucket_name must not be blank")
	}
	return nil
}
