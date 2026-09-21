package resource

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

var (
	_ resource.Resource                = &BYOCCredentialResource{}
	_ resource.ResourceWithImportState = &BYOCCredentialResource{}

	awsInstanceProfileARNPattern = regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:instance-profile/[A-Za-z0-9+=,.@_/-]+$`)
	awsRoleARNPattern            = regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/[A-Za-z0-9+=,.@_/-]+$`)
)

type BYOCCredentialResource struct {
	client *client.FormationClient
}

func NewBYOCCredentialResource() resource.Resource {
	return &BYOCCredentialResource{}
}

type BYOCCredentialResourceModel struct {
	ID                      types.Int64  `tfsdk:"id"`
	CloudProvider           types.String `tfsdk:"cloud_provider"`
	Name                    types.String `tfsdk:"name"`
	Region                  types.String `tfsdk:"region"`
	BucketName              types.String `tfsdk:"bucket_name"`
	DataCredentialARN       types.String `tfsdk:"data_credential_arn"`
	DeploymentCredentialARN types.String `tfsdk:"deployment_credential_arn"`
	ExternalID              types.String `tfsdk:"external_id"`
	WarehouseCount          types.Int64  `tfsdk:"warehouse_count"`
	WarehouseIDs            types.List   `tfsdk:"warehouse_ids"`
	CreatedAt               types.String `tfsdk:"created_at"`
	UpdatedAt               types.String `tfsdk:"updated_at"`
}

func (r *BYOCCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_byoc_credential"
}

func (r *BYOCCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "Registers AWS storage and deployment credentials for an advanced BYOC warehouse.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "VeloDB credential configuration ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"cloud_provider": schema.StringAttribute{
				Description:   "Cloud provider. Only aws is supported in this release.",
				Required:      true,
				Validators:    []validator.String{stringvalidator.OneOf("aws")},
				PlanModifiers: replace,
			},
			"name": schema.StringAttribute{
				Description:   "Credential configuration name.",
				Required:      true,
				Validators:    []validator.String{stringvalidator.LengthBetween(1, 32)},
				PlanModifiers: replace,
			},
			"region": schema.StringAttribute{
				Description:   "AWS region.",
				Required:      true,
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: replace,
			},
			"bucket_name": schema.StringAttribute{
				Description:   "S3 bucket used by the warehouse.",
				Required:      true,
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: replace,
			},
			"data_credential_arn": schema.StringAttribute{
				Description: "AWS IAM instance-profile ARN used to access warehouse data.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(awsInstanceProfileARNPattern, "must be a commercial AWS IAM instance-profile ARN"),
				},
				PlanModifiers: replace,
			},
			"deployment_credential_arn": schema.StringAttribute{
				Description: "AWS IAM role ARN used by VeloDB to deploy warehouse infrastructure.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(awsRoleARNPattern, "must be a commercial AWS IAM role ARN"),
				},
				PlanModifiers: replace,
			},
			"external_id":     schema.StringAttribute{Description: "External ID validated by VeloDB.", Computed: true},
			"warehouse_count": schema.Int64Attribute{Description: "Number of warehouses using this credential configuration.", Computed: true},
			"warehouse_ids": schema.ListAttribute{
				Description: "Warehouses using this credential configuration.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"created_at": schema.StringAttribute{Description: "Creation time in RFC 3339 format.", Computed: true},
			"updated_at": schema.StringAttribute{Description: "Last update time in RFC 3339 format.", Computed: true},
		},
	}
}

func (r *BYOCCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.FormationClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.FormationClient, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *BYOCCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BYOCCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.CreateCloudSettingCredential(ctx, plan.CloudProvider.ValueString(), &client.CreateCloudSettingCredentialRequest{
		Name:                    plan.Name.ValueString(),
		Region:                  plan.Region.ValueString(),
		BucketName:              plan.BucketName.ValueString(),
		DataCredentialARN:       plan.DataCredentialARN.ValueString(),
		DeploymentCredentialARN: plan.DeploymentCredentialARN.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(userError("creating BYOC credential configuration", err))
		return
	}

	plan.ID = types.Int64Value(result.CredentialID)
	found, err := r.read(ctx, &plan, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(userError("reading BYOC credential configuration after creation", err))
		return
	}
	if !found {
		resp.Diagnostics.AddError("Credential configuration disappeared after creation", "VeloDB created the credential configuration but it could not be read back.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BYOCCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BYOCCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, err := r.read(ctx, &state, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(userError("reading BYOC credential configuration", err))
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BYOCCredentialResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("BYOC credential configuration cannot be updated", "Change to this resource must replace it because the VeloDB API has no update operation.")
}

func (r *BYOCCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BYOCCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteCloudSettingCredential(ctx, state.CloudProvider.ValueString(), state.ID.ValueInt64()); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError(userError("deleting BYOC credential configuration", err))
	}
}

func (r *BYOCCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	cloudProvider, id, err := parseBYOCRegistrationImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The VeloDB client is not available during import.")
		return
	}
	if _, err := r.client.GetCloudSettingCredential(ctx, cloudProvider, id); err != nil {
		resp.Diagnostics.AddError(userError("importing BYOC credential configuration", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cloud_provider"), cloudProvider)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *BYOCCredentialResource) read(ctx context.Context, state *BYOCCredentialResourceModel, diags *diag.Diagnostics) (bool, error) {
	item, err := r.client.GetCloudSettingCredential(ctx, state.CloudProvider.ValueString(), state.ID.ValueInt64())
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return false, nil
		}
		return false, err
	}

	state.ID = types.Int64Value(item.CredentialID)
	state.CloudProvider = types.StringValue(item.CloudProvider)
	state.Name = types.StringValue(item.Name)
	state.Region = types.StringValue(item.Region)
	state.BucketName = types.StringValue(item.BucketName)
	state.DataCredentialARN = types.StringValue(item.DataCredentialARN)
	state.DeploymentCredentialARN = types.StringValue(item.DeploymentCredentialARN)
	state.ExternalID = stringOrNull(item.ExternalID)
	state.WarehouseCount = types.Int64Value(int64(item.WarehouseCount))
	warehouseIDs, warehouseDiags := types.ListValueFrom(ctx, types.StringType, item.WarehouseIDs)
	diags.Append(warehouseDiags...)
	state.WarehouseIDs = warehouseIDs
	state.CreatedAt = timeOrNull(item.CreatedAt)
	state.UpdatedAt = timeOrNull(item.UpdatedAt)
	return true, nil
}

func parseBYOCRegistrationImportID(importID string) (string, int64, error) {
	parts := strings.Split(strings.TrimSpace(importID), "/")
	if len(parts) != 2 || parts[0] != "aws" || strings.TrimSpace(parts[1]) == "" {
		return "", 0, fmt.Errorf("expected format: aws/<id>")
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id <= 0 {
		return "", 0, fmt.Errorf("expected format: aws/<positive numeric id>")
	}
	return parts[0], id, nil
}

func timeOrNull(value *time.Time) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(value.Format(time.RFC3339))
}
