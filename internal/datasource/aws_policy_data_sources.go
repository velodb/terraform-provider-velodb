package datasource

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource = &AWSAssumeRolePolicyDataSource{}
	_ datasource.DataSource = &AWSCrossAccountPolicyDataSource{}
	_ datasource.DataSource = &AWSDataAccessAssumeRolePolicyDataSource{}
	_ datasource.DataSource = &AWSDataAccessPolicyDataSource{}
	_ datasource.DataSource = &AWSKMSKeyPolicyDataSource{}
)

type AWSAssumeRolePolicyDataSource struct{}

type AWSAssumeRolePolicyDataSourceModel struct {
	PrincipalARN types.String `tfsdk:"principal_arn"`
	ExternalID   types.String `tfsdk:"external_id"`
	JSON         types.String `tfsdk:"json"`
}

func NewAWSAssumeRolePolicyDataSource() datasource.DataSource {
	return &AWSAssumeRolePolicyDataSource{}
}

func (d *AWSAssumeRolePolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws_assume_role_policy"
}

func (d *AWSAssumeRolePolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Generates the trust policy for the AWS deployment role assumed by VeloDB Cloud.",
		Attributes: map[string]schema.Attribute{
			"principal_arn": schema.StringAttribute{
				Description: "VeloDB AWS deployment-assumer role ARN returned by velodb_byoc_prerequisites.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(awsRoleARNPattern, "must be a commercial AWS IAM role ARN"),
				},
			},
			"external_id": schema.StringAttribute{
				Description: "Organization external ID returned by velodb_byoc_prerequisites.",
				Required:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"json": schema.StringAttribute{Description: "AWS IAM policy document as JSON.", Computed: true},
		},
	}
}

func (d *AWSAssumeRolePolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AWSAssumeRolePolicyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value, err := buildAWSAssumeRolePolicy(data.PrincipalARN.ValueString(), data.ExternalID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid AWS assume-role policy input", err.Error())
		return
	}
	data.JSON = types.StringValue(value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type AWSCrossAccountPolicyDataSource struct{}

type AWSCrossAccountPolicyDataSourceModel struct {
	BucketName        types.String `tfsdk:"bucket_name"`
	DataCredentialARN types.String `tfsdk:"data_credential_arn"`
	JSON              types.String `tfsdk:"json"`
}

func NewAWSCrossAccountPolicyDataSource() datasource.DataSource {
	return &AWSCrossAccountPolicyDataSource{}
}

func (d *AWSCrossAccountPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws_crossaccount_policy"
}

func (d *AWSCrossAccountPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Generates the AWS deployment-role permissions required by VeloDB Cloud.",
		Attributes: map[string]schema.Attribute{
			"bucket_name": schema.StringAttribute{
				Description: "S3 bucket used by the BYOC warehouse.",
				Required:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"data_credential_arn": schema.StringAttribute{
				Description: "AWS IAM instance-profile ARN used by warehouse instances.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(awsInstanceProfileARNPattern, "must be a commercial AWS IAM instance-profile ARN"),
				},
			},
			"json": schema.StringAttribute{Description: "AWS IAM policy document as JSON.", Computed: true},
		},
	}
}

func (d *AWSCrossAccountPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AWSCrossAccountPolicyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value, err := buildAWSCrossAccountPolicy(data.BucketName.ValueString(), data.DataCredentialARN.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid AWS cross-account policy input", err.Error())
		return
	}
	data.JSON = types.StringValue(value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type AWSDataAccessAssumeRolePolicyDataSource struct{}

type AWSDataAccessAssumeRolePolicyDataSourceModel struct {
	RoleARN types.String `tfsdk:"role_arn"`
	JSON    types.String `tfsdk:"json"`
}

func NewAWSDataAccessAssumeRolePolicyDataSource() datasource.DataSource {
	return &AWSDataAccessAssumeRolePolicyDataSource{}
}

func (d *AWSDataAccessAssumeRolePolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws_data_access_assume_role_policy"
}

func (d *AWSDataAccessAssumeRolePolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Generates the EC2 and self-assumption trust policy for the AWS data-access role.",
		Attributes: map[string]schema.Attribute{
			"role_arn": schema.StringAttribute{
				Description: "AWS IAM data-access role ARN.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(awsRoleARNPattern, "must be a commercial AWS IAM role ARN"),
				},
			},
			"json": schema.StringAttribute{Description: "AWS IAM policy document as JSON.", Computed: true},
		},
	}
}

func (d *AWSDataAccessAssumeRolePolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AWSDataAccessAssumeRolePolicyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value, err := buildAWSDataAccessAssumeRolePolicy(data.RoleARN.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid AWS data-access assume-role policy input", err.Error())
		return
	}
	data.JSON = types.StringValue(value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type AWSDataAccessPolicyDataSource struct{}

type AWSDataAccessPolicyDataSourceModel struct {
	BucketName types.String `tfsdk:"bucket_name"`
	RoleARN    types.String `tfsdk:"role_arn"`
	TDEKMSARN  types.String `tfsdk:"tde_kms_arn"`
	JSON       types.String `tfsdk:"json"`
}

func NewAWSDataAccessPolicyDataSource() datasource.DataSource {
	return &AWSDataAccessPolicyDataSource{}
}

func (d *AWSDataAccessPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws_data_access_policy"
}

func (d *AWSDataAccessPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Generates the S3, self-assumption, and optional TDE KMS permissions for the AWS data-access role.",
		Attributes: map[string]schema.Attribute{
			"bucket_name": schema.StringAttribute{
				Description: "S3 bucket used by the BYOC warehouse.",
				Required:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"role_arn": schema.StringAttribute{
				Description: "AWS IAM data-access role ARN.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(awsRoleARNPattern, "must be a commercial AWS IAM role ARN"),
				},
			},
			"tde_kms_arn": schema.StringAttribute{
				Description: "Optional AWS KMS key ARN used for transparent data encryption.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(awsKMSKeyARNPattern, "must be a commercial AWS KMS key ARN"),
				},
			},
			"json": schema.StringAttribute{Description: "AWS IAM policy document as JSON.", Computed: true},
		},
	}
}

func (d *AWSDataAccessPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AWSDataAccessPolicyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value, err := buildAWSDataAccessPolicy(data.BucketName.ValueString(), data.RoleARN.ValueString(), data.TDEKMSARN.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid AWS data-access policy input", err.Error())
		return
	}
	data.JSON = types.StringValue(value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type AWSKMSKeyPolicyDataSource struct{}

type AWSKMSKeyPolicyDataSourceModel struct {
	UseTDE            types.Bool   `tfsdk:"use_tde"`
	UseEBS            types.Bool   `tfsdk:"use_ebs"`
	DataRoleARN       types.String `tfsdk:"data_role_arn"`
	DeploymentRoleARN types.String `tfsdk:"deployment_role_arn"`
	JSON              types.String `tfsdk:"json"`
}

func NewAWSKMSKeyPolicyDataSource() datasource.DataSource {
	return &AWSKMSKeyPolicyDataSource{}
}

func (d *AWSKMSKeyPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws_kms_key_policy"
}

func (d *AWSKMSKeyPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Generates the resource-based KMS key policy VeloDB Cloud requires on the customer-provided KMS key registered as a velodb_encryption_key.",
		Attributes: map[string]schema.Attribute{
			"use_tde": schema.BoolAttribute{
				Description: "Grant the data-access role transparent data encryption (TDE) permissions. Requires data_role_arn.",
				Optional:    true,
			},
			"use_ebs": schema.BoolAttribute{
				Description: "Grant the deployment role EBS volume encryption permissions. Requires deployment_role_arn.",
				Optional:    true,
			},
			"data_role_arn": schema.StringAttribute{
				Description: "AWS IAM data-access role ARN. Required when use_tde is true.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(awsRoleARNPattern, "must be a commercial AWS IAM role ARN"),
				},
			},
			"deployment_role_arn": schema.StringAttribute{
				Description: "AWS IAM deployment role ARN. Required when use_ebs is true.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(awsRoleARNPattern, "must be a commercial AWS IAM role ARN"),
				},
			},
			"json": schema.StringAttribute{Description: "AWS KMS key policy document as JSON.", Computed: true},
		},
	}
}

func (d *AWSKMSKeyPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AWSKMSKeyPolicyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value, err := buildAWSKMSKeyPolicy(data.UseTDE.ValueBool(), data.UseEBS.ValueBool(), data.DataRoleARN.ValueString(), data.DeploymentRoleARN.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid AWS KMS key policy input", err.Error())
		return
	}
	data.JSON = types.StringValue(value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
