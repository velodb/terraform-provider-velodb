package datasource

import (
	"encoding/json"
	"strings"
	"testing"
)

const (
	testRoleARN            = "arn:aws:iam::111122223333:role/velodb-data"
	testInstanceProfileARN = "arn:aws:iam::111122223333:instance-profile/velodb-data"
)

func TestBuildAWSAssumeRolePolicy(t *testing.T) {
	policyJSON, err := buildAWSAssumeRolePolicy(awsDeploymentAssumerRoleARN, "external-123")
	if err != nil {
		t.Fatal(err)
	}
	policy := decodeAWSPolicy(t, policyJSON)
	statement := requireStatement(t, policy, "sts:AssumeRole")
	if statement.Principal["AWS"] != awsDeploymentAssumerRoleARN {
		t.Fatalf("unexpected principal: %v", statement.Principal)
	}
	if got := statement.Condition["StringEquals"]["sts:ExternalId"]; got != "external-123" {
		t.Fatalf("external ID = %q", got)
	}
}

func TestBuildAWSDataAccessAssumeRolePolicy(t *testing.T) {
	policyJSON, err := buildAWSDataAccessAssumeRolePolicy(testRoleARN)
	if err != nil {
		t.Fatal(err)
	}
	policy := decodeAWSPolicy(t, policyJSON)
	if len(policy.Statements) != 2 {
		t.Fatalf("statements = %d, want 2", len(policy.Statements))
	}
	if got := policy.Statements[0].Principal["Service"]; got != "ec2.amazonaws.com" {
		t.Fatalf("EC2 principal = %q", got)
	}
	self := policy.Statements[1]
	if got := self.Principal["AWS"]; got != "arn:aws:iam::111122223333:root" {
		t.Fatalf("self-assume principal = %q", got)
	}
	if got := self.Condition["StringEquals"]["aws:PrincipalArn"]; got != testRoleARN {
		t.Fatalf("self-assume condition = %q", got)
	}
}

func TestBuildAWSDataAccessPolicy(t *testing.T) {
	withoutKMS, err := buildAWSDataAccessPolicy("velodb-data", testRoleARN, "")
	if err != nil {
		t.Fatal(err)
	}
	policy := decodeAWSPolicy(t, withoutKMS)
	if len(policy.Statements) != 3 {
		t.Fatalf("statements without KMS = %d, want 3", len(policy.Statements))
	}
	assertResources(t, requireStatement(t, policy, "s3:ListBucket"), "arn:aws:s3:::velodb-data")
	assertResources(t, requireStatement(t, policy, "s3:GetObject"), "arn:aws:s3:::velodb-data/*")
	assertResources(t, requireStatement(t, policy, "sts:AssumeRole"), testRoleARN)

	const kmsARN = "arn:aws:kms:us-east-1:111122223333:key/12345678-1234-1234-1234-123456789012"
	withKMS, err := buildAWSDataAccessPolicy("velodb-data", testRoleARN, kmsARN)
	if err != nil {
		t.Fatal(err)
	}
	kmsPolicy := decodeAWSPolicy(t, withKMS)
	if len(kmsPolicy.Statements) != 4 {
		t.Fatalf("statements with KMS = %d, want 4", len(kmsPolicy.Statements))
	}
	kms := requireStatement(t, kmsPolicy, "kms:GenerateDataKey*")
	if kms.Sid != "KMSAccess" {
		t.Fatalf("KMS Sid = %q", kms.Sid)
	}
	assertResources(t, kms, kmsARN)
}

func TestBuildAWSCrossAccountPolicy(t *testing.T) {
	policyJSON, err := buildAWSCrossAccountPolicy("velodb-data", testInstanceProfileARN)
	if err != nil {
		t.Fatal(err)
	}
	policy := decodeAWSPolicy(t, policyJSON)
	if len(policy.Statements) != 17 {
		t.Fatalf("statements = %d, want 17", len(policy.Statements))
	}
	compact, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(compact) > 6144 {
		t.Fatalf("deployment policy is %d non-whitespace bytes; AWS managed policies allow 6144", len(compact))
	}

	assertResources(t, requireStatement(t, policy, "s3:PutLifecycleConfiguration"), "arn:aws:s3:::velodb-data")
	iamRead := requireStatement(t, policy, "iam:GetInstanceProfile")
	assertResources(t, iamRead, testInstanceProfileARN, testRoleARN)
	passRole := requireStatement(t, policy, "iam:PassRole")
	assertResources(t, passRole, testRoleARN)
	if got := passRole.Condition["StringEquals"]["iam:PassedToService"]; got != "ec2.amazonaws.com" {
		t.Fatalf("PassRole service = %q", got)
	}
	if got := requireStatement(t, policy, "ec2:DeleteVpcEndpoints").Condition["StringEquals"]["ec2:ResourceTag/resource-created-by"]; got != "velodb" {
		t.Fatalf("VPC endpoint tag = %q", got)
	}
	assertResources(t, requireStatement(t, policy, "kms:CreateGrant"), "*")

	for _, action := range []string{
		"ec2:RunInstances", "ec2:TerminateInstances", "ec2:CreateVpcEndpoint",
		"elasticloadbalancing:CreateLoadBalancer", "elasticloadbalancing:RegisterTargets",
		"iam:CreateServiceLinkedRole", "kms:GetKeyPolicy",
	} {
		requireStatement(t, policy, action)
	}

	again, err := buildAWSCrossAccountPolicy("velodb-data", testInstanceProfileARN)
	if err != nil {
		t.Fatal(err)
	}
	if policyJSON != again {
		t.Fatal("policy output is not deterministic")
	}
}

func TestAWSPolicyBuildersRejectInvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{name: "blank external ID", fn: func() error { _, err := buildAWSAssumeRolePolicy(awsDeploymentAssumerRoleARN, " "); return err }},
		{name: "invalid principal", fn: func() error {
			_, err := buildAWSAssumeRolePolicy("arn:aws:iam::111122223333:root", "external")
			return err
		}},
		{name: "invalid data role", fn: func() error { _, err := buildAWSDataAccessAssumeRolePolicy(testInstanceProfileARN); return err }},
		{name: "blank bucket", fn: func() error { _, err := buildAWSDataAccessPolicy(" ", testRoleARN, ""); return err }},
		{name: "invalid KMS ARN", fn: func() error { _, err := buildAWSDataAccessPolicy("velodb-data", testRoleARN, "alias/key"); return err }},
		{name: "role instead of instance profile", fn: func() error { _, err := buildAWSCrossAccountPolicy("velodb-data", testRoleARN); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func decodeAWSPolicy(t *testing.T, value string) awsPolicyDocument {
	t.Helper()
	var policy awsPolicyDocument
	if err := json.Unmarshal([]byte(value), &policy); err != nil {
		t.Fatalf("decode policy: %v", err)
	}
	if policy.Version != awsPolicyVersion {
		t.Fatalf("version = %q", policy.Version)
	}
	return policy
}

func requireStatement(t *testing.T, policy awsPolicyDocument, action string) awsPolicyStatement {
	t.Helper()
	for _, statement := range policy.Statements {
		for _, candidate := range statement.Actions {
			if candidate == action {
				return statement
			}
		}
	}
	t.Fatalf("missing action %q", action)
	return awsPolicyStatement{}
}

func assertResources(t *testing.T, statement awsPolicyStatement, want ...string) {
	t.Helper()
	if strings.Join(statement.Resources, "\n") != strings.Join(want, "\n") {
		t.Fatalf("resources = %v, want %v", statement.Resources, want)
	}
}
