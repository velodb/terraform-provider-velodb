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

func TestBuildAWSKMSKeyPolicy(t *testing.T) {
	dataRole := "arn:aws:iam::111122223333:role/velodb-data"
	deployRole := "arn:aws:iam::111122223333:role/velodb-deploy"

	if _, err := buildAWSKMSKeyPolicy(false, false, "", ""); err == nil {
		t.Fatal("expected error when neither use is set")
	}
	if _, err := buildAWSKMSKeyPolicy(true, false, "", ""); err == nil {
		t.Fatal("expected error when use_tde set without data_role_arn")
	}
	if _, err := buildAWSKMSKeyPolicy(false, true, "", ""); err == nil {
		t.Fatal("expected error when use_ebs set without deployment_role_arn")
	}
	if _, err := buildAWSKMSKeyPolicy(true, true, "arn:aws:iam::111122223333:role/a", "arn:aws:iam::999988887777:role/b"); err == nil {
		t.Fatal("expected error when data and deployment roles are in different accounts")
	}

	policyJSON, err := buildAWSKMSKeyPolicy(true, true, dataRole, deployRole)
	if err != nil {
		t.Fatal(err)
	}
	policy := decodeAWSPolicy(t, policyJSON)

	root := requireSid(t, policy, "EnableRootAccount")
	if root.Principal["AWS"] != "arn:aws:iam::111122223333:root" {
		t.Fatalf("root principal = %v", root.Principal)
	}
	if len(root.Actions) != 1 || root.Actions[0] != "kms:*" {
		t.Fatalf("root actions = %v", root.Actions)
	}

	tde := requireSid(t, policy, "AllowSelectDBTdeAccess")
	if tde.Principal["AWS"] != dataRole {
		t.Fatalf("tde principal = %v", tde.Principal)
	}
	assertActions(t, tde, "kms:Encrypt", "kms:Decrypt", "kms:GenerateDataKey*", "kms:DescribeKey")
	if tde.Condition != nil {
		t.Fatalf("tde must not have a condition: %v", tde.Condition)
	}

	ebs := requireSid(t, policy, "AllowSelectDBEbsAccess")
	if ebs.Principal["AWS"] != deployRole {
		t.Fatalf("ebs principal = %v", ebs.Principal)
	}
	assertActions(t, ebs, "kms:Decrypt", "kms:GenerateDataKey*", "kms:CreateGrant", "kms:ReEncrypt*", "kms:DescribeKey")
	if got := ebs.Condition["ForAnyValue:StringLike"]["kms:ViaService"]; got != "ec2.*.amazonaws.com" {
		t.Fatalf("ebs kms:ViaService = %q", got)
	}

	// TDE-only: no EBS statement.
	tdeOnly := decodeAWSPolicy(t, mustBuildKMSKeyPolicy(t, true, false, dataRole, ""))
	if len(tdeOnly.Statements) != 2 {
		t.Fatalf("tde-only statements = %d, want 2", len(tdeOnly.Statements))
	}
}

func mustBuildKMSKeyPolicy(t *testing.T, useTDE, useEBS bool, dataRole, deployRole string) string {
	t.Helper()
	value, err := buildAWSKMSKeyPolicy(useTDE, useEBS, dataRole, deployRole)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func requireSid(t *testing.T, policy awsPolicyDocument, sid string) awsPolicyStatement {
	t.Helper()
	for _, statement := range policy.Statements {
		if statement.Sid == sid {
			return statement
		}
	}
	t.Fatalf("missing statement %q", sid)
	return awsPolicyStatement{}
}

func assertActions(t *testing.T, statement awsPolicyStatement, want ...string) {
	t.Helper()
	if strings.Join(statement.Actions, "\n") != strings.Join(want, "\n") {
		t.Fatalf("actions = %v, want %v", statement.Actions, want)
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
