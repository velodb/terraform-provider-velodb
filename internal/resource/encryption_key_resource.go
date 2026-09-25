package resource

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

var (
	_ resource.Resource                   = &EncryptionKeyResource{}
	_ resource.ResourceWithImportState    = &EncryptionKeyResource{}
	_ resource.ResourceWithValidateConfig = &EncryptionKeyResource{}

	awsKMSKeyARNPattern = regexp.MustCompile(`^arn:aws:kms:[a-z0-9-]+:[0-9]{12}:key/[A-Za-z0-9-]+$`)
)

type EncryptionKeyResource struct {
	client *client.FormationClient
}

func NewEncryptionKeyResource() resource.Resource {
	return &EncryptionKeyResource{}
}

type EncryptionKeyResourceModel struct {
	ID             types.Int64  `tfsdk:"id"`
	CloudProvider  types.String `tfsdk:"cloud_provider"`
	Name           types.String `tfsdk:"name"`
	KeyARN         types.String `tfsdk:"key_arn"`
	UseTDE         types.Bool   `tfsdk:"use_tde"`
	UseEBS         types.Bool   `tfsdk:"use_ebs"`
	Region         types.String `tfsdk:"region"`
	WarehouseCount types.Int64  `tfsdk:"warehouse_count"`
	WarehouseIDs   types.List   `tfsdk:"warehouse_ids"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (r *EncryptionKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_encryption_key"
}

func (r *EncryptionKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replaceStr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	replaceBool := []planmodifier.Bool{boolplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "Registers a customer-provided AWS KMS key with VeloDB for use as a warehouse TDE and/or EBS encryption key. The KMS key policy must grant VeloDB the required access; use the velodb_aws_kms_key_policy data source to generate it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "VeloDB encryption key configuration ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"cloud_provider": schema.StringAttribute{
				Description:   "Cloud provider. Only aws is supported in this release.",
				Required:      true,
				Validators:    []validator.String{stringvalidator.OneOf("aws")},
				PlanModifiers: replaceStr,
			},
			"name": schema.StringAttribute{
				Description:   "Encryption key configuration name. Unique within the organization.",
				Required:      true,
				Validators:    []validator.String{stringvalidator.LengthBetween(1, 32)},
				PlanModifiers: replaceStr,
			},
			"key_arn": schema.StringAttribute{
				Description: "AWS KMS key ARN.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(awsKMSKeyARNPattern, "must be a commercial AWS KMS key ARN"),
				},
				PlanModifiers: replaceStr,
			},
			"use_tde": schema.BoolAttribute{
				Description:   "Allow this key to be used for transparent data encryption (TDE) of warehouse data. At least one of use_tde or use_ebs must be true.",
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				PlanModifiers: replaceBool,
			},
			"use_ebs": schema.BoolAttribute{
				Description:   "Allow this key to be used for encrypting warehouse EBS volumes. At least one of use_tde or use_ebs must be true.",
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				PlanModifiers: replaceBool,
			},
			"region":          schema.StringAttribute{Description: "AWS region derived from the KMS key ARN.", Computed: true},
			"warehouse_count": schema.Int64Attribute{Description: "Number of warehouses using this encryption key.", Computed: true},
			"warehouse_ids": schema.ListAttribute{
				Description: "Warehouses using this encryption key.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"created_at": schema.StringAttribute{Description: "Creation time.", Computed: true},
			"updated_at": schema.StringAttribute{Description: "Last update time.", Computed: true},
		},
	}
}

func (r *EncryptionKeyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config EncryptionKeyResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Treat null as false; both false is invalid.
	if !config.UseTDE.ValueBool() && !config.UseEBS.ValueBool() {
		resp.Diagnostics.AddError(
			"Encryption key has no use",
			"At least one of use_tde or use_ebs must be true.",
		)
	}
}

func (r *EncryptionKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EncryptionKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EncryptionKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.CreateEncryptionKey(ctx, plan.CloudProvider.ValueString(), &client.CreateEncryptionKeyRequest{
		Name:   plan.Name.ValueString(),
		KeyARN: plan.KeyARN.ValueString(),
		UseTDE: boolToAPIFlag(plan.UseTDE),
		UseEBS: boolToAPIFlag(plan.UseEBS),
	})
	if err != nil {
		resp.Diagnostics.AddError(userError("creating encryption key", err))
		return
	}

	plan.ID = types.Int64Value(result.EncryptionKeyID)
	found, err := r.read(ctx, &plan, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(userError("reading encryption key after creation", err))
		return
	}
	if !found {
		resp.Diagnostics.AddError("Encryption key disappeared after creation", "VeloDB created the encryption key but it could not be read back.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EncryptionKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EncryptionKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, err := r.read(ctx, &state, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(userError("reading encryption key", err))
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *EncryptionKeyResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Encryption key cannot be updated", "Change to this resource must replace it because the VeloDB API has no update operation.")
}

func (r *EncryptionKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EncryptionKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteEncryptionKey(ctx, state.CloudProvider.ValueString(), state.ID.ValueInt64()); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError(userError("deleting encryption key", err))
	}
}

func (r *EncryptionKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	cloudProvider, id, err := parseBYOCImportID(req.ID, "encryption_key_id")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The VeloDB client is not available during import.")
		return
	}
	if _, err := r.client.GetEncryptionKey(ctx, cloudProvider, id); err != nil {
		resp.Diagnostics.AddError(userError("importing encryption key", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cloud_provider"), cloudProvider)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *EncryptionKeyResource) read(ctx context.Context, state *EncryptionKeyResourceModel, diags *diag.Diagnostics) (bool, error) {
	item, err := r.client.GetEncryptionKey(ctx, state.CloudProvider.ValueString(), state.ID.ValueInt64())
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return false, nil
		}
		return false, err
	}

	state.ID = types.Int64Value(item.EncryptionKeyID)
	state.CloudProvider = types.StringValue(item.CloudProvider)
	state.Name = types.StringValue(item.Name)
	state.KeyARN = types.StringValue(item.KeyARN)
	state.UseTDE = types.BoolValue(item.UseTDE)
	state.UseEBS = types.BoolValue(item.UseEBS)
	state.Region = types.StringValue(item.Region)
	state.WarehouseCount = types.Int64Value(int64(item.WarehouseCount))
	warehouseIDs, warehouseDiags := types.ListValueFrom(ctx, types.StringType, item.WarehouseIDs)
	diags.Append(warehouseDiags...)
	state.WarehouseIDs = warehouseIDs
	state.CreatedAt = stringOrNull(item.CreatedAt)
	state.UpdatedAt = stringOrNull(item.UpdatedAt)
	return true, nil
}

// boolToAPIFlag converts a Terraform bool into the 0/1 integer the VeloDB API
// expects for encryption key uses.
func boolToAPIFlag(value types.Bool) *int {
	flag := 0
	if value.ValueBool() {
		flag = 1
	}
	return &flag
}
