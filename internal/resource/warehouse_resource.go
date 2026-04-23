package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

var (
	_ resource.Resource                = &WarehouseResource{}
	_ resource.ResourceWithImportState = &WarehouseResource{}
)

type WarehouseResource struct {
	client *client.FormationClient
}

func NewWarehouseResource() resource.Resource {
	return &WarehouseResource{}
}

// --- Terraform model ---

type WarehouseResourceModel struct {
	ID                       types.String  `tfsdk:"id"`
	Name                     types.String  `tfsdk:"name"`
	DeploymentMode           types.String  `tfsdk:"deployment_mode"`
	CloudProvider            types.String  `tfsdk:"cloud_provider"`
	Region                   types.String  `tfsdk:"region"`
	CreateMode               types.String  `tfsdk:"create_mode"`
	VpcMode                  types.String  `tfsdk:"vpc_mode"`
	VpcID                    types.String  `tfsdk:"vpc_id"`
	CredentialID             types.Int64   `tfsdk:"credential_id"`
	NetworkConfigID          types.Int64   `tfsdk:"network_config_id"`
	BucketName               types.String  `tfsdk:"bucket_name"`
	DataCredentialArn        types.String  `tfsdk:"data_credential_arn"`
	DeploymentCredentialArn  types.String  `tfsdk:"deployment_credential_arn"`
	SubnetID                 types.String  `tfsdk:"subnet_id"`
	SecurityGroupID          types.String  `tfsdk:"security_group_id"`
	EndpointID               types.String  `tfsdk:"endpoint_id"`
	CoreVersion              types.String  `tfsdk:"core_version"`
	AdminPassword            types.String  `tfsdk:"admin_password"`
	AdminPasswordVersion     types.Int64   `tfsdk:"admin_password_version"`
	MaintainabilityStartTime types.String  `tfsdk:"maintainability_start_time"`
	MaintainabilityEndTime   types.String  `tfsdk:"maintainability_end_time"`
	AdvancedSettings         types.String  `tfsdk:"advanced_settings"`
	Tags                     types.Map     `tfsdk:"tags"`
	InitialCluster           types.List    `tfsdk:"initial_cluster"`
	Timeouts                 timeouts.Value `tfsdk:"timeouts"`
	// Computed
	Status     types.String `tfsdk:"status"`
	Zone       types.String `tfsdk:"zone"`
	PayType    types.String `tfsdk:"pay_type"`
	CreatedAt  types.String `tfsdk:"created_at"`
	ExpireTime types.String `tfsdk:"expire_time"`
	ByocSetup  types.List   `tfsdk:"byoc_setup"`
}

type InitialClusterModel struct {
	Name          types.String `tfsdk:"name"`
	Zone          types.String `tfsdk:"zone"`
	ComputeVcpu   types.Int64  `tfsdk:"compute_vcpu"`
	CacheGb       types.Int64  `tfsdk:"cache_gb"`
	BillingModel types.String `tfsdk:"billing_model"`
	Period        types.Int64  `tfsdk:"period"`
	PeriodUnit    types.String `tfsdk:"period_unit"`
	AutoPause     types.List   `tfsdk:"auto_pause"`
}

type AutoPauseModel struct {
	Enabled            types.Bool  `tfsdk:"enabled"`
	IdleTimeoutMinutes types.Int64 `tfsdk:"idle_timeout_minutes"`
}

type ByocSetupModel struct {
	Token                 types.String `tfsdk:"token"`
	ShellCommand          types.String `tfsdk:"shell_command"`
	ShellCommandForNewVpc types.String `tfsdk:"shell_command_for_new_vpc"`
	URL                   types.String `tfsdk:"url"`
	DocURL                types.String `tfsdk:"doc_url"`
	URLForNewVpc          types.String `tfsdk:"url_for_new_vpc"`
	DocURLForNewVpc       types.String `tfsdk:"doc_url_for_new_vpc"`
}

// --- Schema ---

func (r *WarehouseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warehouse"
}

func (r *WarehouseResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a VeloDB Cloud warehouse.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Warehouse identifier.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Warehouse display name.",
				Required:    true,
			},
			"deployment_mode": schema.StringAttribute{
				Description: "Deployment mode: BYOC or SAAS.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cloud_provider": schema.StringAttribute{
				Description: "Cloud provider (e.g., aws, aliyun).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region": schema.StringAttribute{
				Description: "Cloud region (e.g., us-east-1, cn-beijing).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"create_mode": schema.StringAttribute{
				Description: "BYOC creation mode: Template or Wizard.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"vpc_mode": schema.StringAttribute{
				Description: "VPC mode hint: existing or new.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"vpc_id": schema.StringAttribute{
				Description: "Existing VPC identifier for Template mode.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"credential_id": schema.Int64Attribute{
				Description: "Credential identifier for Wizard mode.",
				Optional:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"network_config_id": schema.Int64Attribute{
				Description: "Network configuration identifier for Wizard mode.",
				Optional:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"bucket_name": schema.StringAttribute{
				Description: "Object storage bucket name for Wizard mode.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"data_credential_arn": schema.StringAttribute{
				Description: "Data plane credential ARN.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"deployment_credential_arn": schema.StringAttribute{
				Description: "Deployment credential ARN.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subnet_id": schema.StringAttribute{
				Description: "Existing subnet identifier.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"security_group_id": schema.StringAttribute{
				Description: "Existing security group identifier.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"endpoint_id": schema.StringAttribute{
				Description: "Existing private endpoint identifier.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"core_version": schema.StringAttribute{
				Description: "Core version. Changing triggers an upgrade.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"admin_password": schema.StringAttribute{
				Description: "Administrator password. Write-only — not stored in state.",
				Optional:    true,
				Sensitive:   true,
			},
			"admin_password_version": schema.Int64Attribute{
				Description: "Increment to trigger a password change.",
				Optional:    true,
			},
			"maintainability_start_time": schema.StringAttribute{
				Description: "Maintenance window start time.",
				Optional:    true,
			},
			"maintainability_end_time": schema.StringAttribute{
				Description: "Maintenance window end time.",
				Optional:    true,
			},
			"advanced_settings": schema.StringAttribute{
				Description: "Advanced settings as a JSON string (use jsonencode).",
				Optional:    true,
			},
			"tags": schema.MapAttribute{
				Description: "Warehouse tags.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{},
			},
			// Computed
			"status": schema.StringAttribute{
				Description: "Current warehouse status.",
				Computed:    true,
			},
			"zone": schema.StringAttribute{
				Description: "Primary availability zone.",
				Computed:    true,
			},
			"pay_type": schema.StringAttribute{
				Description: "Billing type (PostPaid or PrePaid).",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Creation time in ISO 8601 format.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"expire_time": schema.StringAttribute{
				Description: "Expiration time when available.",
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"initial_cluster": schema.ListNestedBlock{
				Description: "Initial cluster created with the warehouse. Create-only.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "Cluster name.",
							Required:    true,
						},
						"zone": schema.StringAttribute{
							Description: "Availability zone.",
							Optional:    true,
						},
						"compute_vcpu": schema.Int64Attribute{
							Description: "Compute capacity in vCPUs.",
							Required:    true,
						},
						"cache_gb": schema.Int64Attribute{
							Description: "Cache capacity in GB.",
							Required:    true,
						},
						"billing_model": schema.StringAttribute{
							Description: "Billing method (e.g., monthly, on_demand).",
							Optional:    true,
						},
						"period": schema.Int64Attribute{
							Description: "Prepaid subscription length.",
							Optional:    true,
						},
						"period_unit": schema.StringAttribute{
							Description: "Period unit: Month, Year, or Week.",
							Optional:    true,
						},
					},
					Blocks: map[string]schema.Block{
						"auto_pause": schema.ListNestedBlock{
							Description: "Auto-pause configuration.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"enabled": schema.BoolAttribute{
										Description: "Whether auto-pause is enabled.",
										Required:    true,
									},
									"idle_timeout_minutes": schema.Int64Attribute{
										Description: "Idle timeout in minutes.",
										Optional:    true,
									},
								},
							},
						},
					},
				},
			},
			"byoc_setup": schema.ListNestedBlock{
				Description: "BYOC setup guidance (computed).",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"token":                    schema.StringAttribute{Computed: true},
						"shell_command":             schema.StringAttribute{Computed: true},
						"shell_command_for_new_vpc": schema.StringAttribute{Computed: true},
						"url":                       schema.StringAttribute{Computed: true},
						"doc_url":                   schema.StringAttribute{Computed: true},
						"url_for_new_vpc":           schema.StringAttribute{Computed: true},
						"doc_url_for_new_vpc":       schema.StringAttribute{Computed: true},
					},
				},
			},
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *WarehouseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// --- CRUD ---

func (r *WarehouseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WarehouseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 45*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Build request
	createReq := &client.CreateWarehouseRequest{
		Name:           plan.Name.ValueString(),
		DeploymentMode: plan.DeploymentMode.ValueString(),
		CloudProvider:  plan.CloudProvider.ValueString(),
		Region:         plan.Region.ValueString(),
	}
	setOptionalString(&createReq.VpcMode, plan.VpcMode)
	setOptionalString(&createReq.CreateMode, plan.CreateMode)
	setOptionalString(&createReq.VpcID, plan.VpcID)
	setOptionalInt64(&createReq.CredentialID, plan.CredentialID)
	setOptionalInt64(&createReq.NetworkConfigID, plan.NetworkConfigID)
	setOptionalString(&createReq.BucketName, plan.BucketName)
	setOptionalString(&createReq.DataCredentialArn, plan.DataCredentialArn)
	setOptionalString(&createReq.DeploymentCredentialArn, plan.DeploymentCredentialArn)
	setOptionalString(&createReq.SubnetID, plan.SubnetID)
	setOptionalString(&createReq.SecurityGroupID, plan.SecurityGroupID)
	setOptionalString(&createReq.EndpointID, plan.EndpointID)
	setOptionalString(&createReq.CoreVersion, plan.CoreVersion)
	setOptionalString(&createReq.AdminPassword, plan.AdminPassword)

	// Tags
	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		tags := make(map[string]string)
		resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Tags = tags
	}

	// Advanced settings
	if !plan.AdvancedSettings.IsNull() && !plan.AdvancedSettings.IsUnknown() {
		var adv map[string]any
		if err := json.Unmarshal([]byte(plan.AdvancedSettings.ValueString()), &adv); err != nil {
			resp.Diagnostics.AddError("Invalid advanced_settings JSON", err.Error())
			return
		}
		createReq.AdvancedSettings = adv
	}

	// Initial cluster
	if !plan.InitialCluster.IsNull() && !plan.InitialCluster.IsUnknown() {
		var clusters []InitialClusterModel
		resp.Diagnostics.Append(plan.InitialCluster.ElementsAs(ctx, &clusters, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if len(clusters) > 0 {
			ic := clusters[0]
			clReq := &client.InitialClusterRequest{
				Name:        ic.Name.ValueString(),
				ComputeVcpu: int(ic.ComputeVcpu.ValueInt64()),
				CacheGb:     int(ic.CacheGb.ValueInt64()),
			}
			setOptionalString(&clReq.Zone, ic.Zone)
			setOptionalString(&clReq.BillingModel, ic.BillingModel)
			setOptionalIntFromInt64(&clReq.Period, ic.Period)
			setOptionalString(&clReq.PeriodUnit, ic.PeriodUnit)

			if !ic.AutoPause.IsNull() && !ic.AutoPause.IsUnknown() {
				var apModels []AutoPauseModel
				resp.Diagnostics.Append(ic.AutoPause.ElementsAs(ctx, &apModels, false)...)
				if resp.Diagnostics.HasError() {
					return
				}
				if len(apModels) > 0 {
					ap := apModels[0]
					apCfg := &client.AutoPauseConfig{
						Enabled: ap.Enabled.ValueBool(),
					}
					setOptionalIntFromInt64(&apCfg.IdleTimeoutMinutes, ap.IdleTimeoutMinutes)
					clReq.AutoPause = apCfg
				}
			}
			createReq.InitialCluster = clReq
		}
	}

	result, err := r.client.CreateWarehouse(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating warehouse", err.Error())
		return
	}

	plan.ID = types.StringValue(result.WarehouseID)

	// Store BYOC setup if returned
	r.setByocSetup(ctx, &plan, result.ByocSetup, &resp.Diagnostics)

	// Wait for warehouse to become Running
	_, err = client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
		wh, err := r.client.GetWarehouse(ctx, result.WarehouseID)
		if err != nil {
			return "", err
		}
		return wh.Status, nil
	}, []string{"Running"}, client.FailedStatuses, createTimeout, 15*time.Second)
	if err != nil {
		resp.Diagnostics.AddWarning("Warehouse created but not yet Running", err.Error())
	}

	// Read back state
	r.readWarehouseIntoState(ctx, result.WarehouseID, &plan, &resp.Diagnostics)

	// Preserve admin_password from plan (can't be read from API)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WarehouseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WarehouseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve admin_password and admin_password_version from prior state (can't read from API)
	priorPassword := state.AdminPassword
	priorPasswordVersion := state.AdminPasswordVersion
	priorInitialCluster := state.InitialCluster
	priorAdvancedSettings := state.AdvancedSettings
	priorTimeouts := state.Timeouts

	r.readWarehouseIntoState(ctx, state.ID.ValueString(), &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	state.AdminPassword = priorPassword
	state.AdminPasswordVersion = priorPasswordVersion
	state.InitialCluster = priorInitialCluster
	state.AdvancedSettings = priorAdvancedSettings
	state.Timeouts = priorTimeouts

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WarehouseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state WarehouseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, 15*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	warehouseID := state.ID.ValueString()

	// Update name and maintenance window via PATCH /warehouses/{id}
	if !plan.Name.Equal(state.Name) ||
		!plan.MaintainabilityStartTime.Equal(state.MaintainabilityStartTime) ||
		!plan.MaintainabilityEndTime.Equal(state.MaintainabilityEndTime) {

		updateReq := &client.UpdateWarehouseRequest{}
		if !plan.Name.Equal(state.Name) {
			s := plan.Name.ValueString()
			updateReq.Name = &s
		}
		if !plan.MaintainabilityStartTime.Equal(state.MaintainabilityStartTime) {
			setOptionalString(&updateReq.MaintainabilityStartTime, plan.MaintainabilityStartTime)
		}
		if !plan.MaintainabilityEndTime.Equal(state.MaintainabilityEndTime) {
			setOptionalString(&updateReq.MaintainabilityEndTime, plan.MaintainabilityEndTime)
		}
		if err := r.client.UpdateWarehouse(ctx, warehouseID, updateReq); err != nil {
			resp.Diagnostics.AddError("Error updating warehouse", err.Error())
			return
		}
	}

	// Update advanced settings via PATCH /warehouses/{id}/settings
	if !plan.AdvancedSettings.Equal(state.AdvancedSettings) {
		settingsReq := &client.UpdateWarehouseSettingsRequest{}
		if !plan.AdvancedSettings.IsNull() && !plan.AdvancedSettings.IsUnknown() {
			var adv map[string]any
			if err := json.Unmarshal([]byte(plan.AdvancedSettings.ValueString()), &adv); err != nil {
				resp.Diagnostics.AddError("Invalid advanced_settings JSON", err.Error())
				return
			}
			settingsReq.AdvancedSettings = adv
		}
		if err := r.client.UpdateWarehouseSettings(ctx, warehouseID, settingsReq); err != nil {
			resp.Diagnostics.AddError("Error updating warehouse settings", err.Error())
			return
		}
	}

	// Trigger version upgrade if core_version changed
	if !plan.CoreVersion.IsNull() && !plan.CoreVersion.IsUnknown() &&
		!plan.CoreVersion.Equal(state.CoreVersion) {
		if err := r.client.UpgradeWarehouse(ctx, warehouseID, plan.CoreVersion.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error upgrading warehouse", err.Error())
			return
		}
		// Wait for upgrade to complete
		_, err := client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
			wh, err := r.client.GetWarehouse(ctx, warehouseID)
			if err != nil {
				return "", err
			}
			return wh.Status, nil
		}, []string{"Running"}, client.FailedStatuses, updateTimeout, 15*time.Second)
		if err != nil {
			resp.Diagnostics.AddWarning("Warehouse upgrade may still be in progress", err.Error())
		}
	}

	// Change password if password changed or version bumped
	if !plan.AdminPassword.IsNull() && !plan.AdminPassword.IsUnknown() &&
		(!plan.AdminPassword.Equal(state.AdminPassword) ||
			!plan.AdminPasswordVersion.Equal(state.AdminPasswordVersion)) {
		if err := r.client.ChangeWarehousePassword(ctx, warehouseID, plan.AdminPassword.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error changing warehouse password", err.Error())
			return
		}
	}

	// Read back state, preserving plan values for write-only/config-only fields
	r.readWarehouseIntoState(ctx, warehouseID, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WarehouseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WarehouseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, 20*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	warehouseID := state.ID.ValueString()

	if err := r.client.DeleteWarehouse(ctx, warehouseID); err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			return // already deleted
		}
		resp.Diagnostics.AddError("Error deleting warehouse", err.Error())
		return
	}

	// Wait for deletion
	_, err := client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
		wh, err := r.client.GetWarehouse(ctx, warehouseID)
		if err != nil {
			return "", err
		}
		return wh.Status, nil
	}, []string{"Deleted"}, nil, deleteTimeout, 15*time.Second)
	if err != nil {
		resp.Diagnostics.AddWarning("Warehouse deletion may still be in progress", err.Error())
	}
}

func (r *WarehouseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// --- Helpers ---

func (r *WarehouseResource) readWarehouseIntoState(ctx context.Context, warehouseID string, state *WarehouseResourceModel, diags *diag.Diagnostics) {
	wh, err := r.client.GetWarehouse(ctx, warehouseID)
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			state.ID = types.StringNull()
			return
		}
		diags.AddError("Error reading warehouse", err.Error())
		return
	}

	state.ID = types.StringValue(wh.WarehouseID)
	state.Name = types.StringValue(wh.Name)
	state.CloudProvider = types.StringValue(wh.CloudProvider)
	state.Region = types.StringValue(wh.Region)
	state.Status = types.StringValue(wh.Status)
	state.Zone = stringOrNull(wh.Zone)
	state.DeploymentMode = stringOrNull(wh.DeploymentMode)
	state.CoreVersion = stringOrNull(wh.CoreVersion)
	state.PayType = stringOrNull(wh.PayType)

	if wh.CreatedAt != nil {
		state.CreatedAt = types.StringValue(wh.CreatedAt.Format(time.RFC3339))
	}
	if wh.ExpireTime != nil {
		state.ExpireTime = types.StringValue(wh.ExpireTime.Format(time.RFC3339))
	} else {
		state.ExpireTime = types.StringNull()
	}

	// Try to get BYOC setup for BYOC warehouses
	if wh.DeploymentMode == "BYOC" || wh.DeploymentMode == "byoc" {
		setup, err := r.client.GetWarehouseByocSetup(ctx, warehouseID)
		if err == nil {
			r.setByocSetup(ctx, state, setup, diags)
		}
	}
}

func (r *WarehouseResource) setByocSetup(ctx context.Context, state *WarehouseResourceModel, setup *client.WarehouseByocSetup, diags *diag.Diagnostics) {
	if setup == nil {
		state.ByocSetup = types.ListNull(types.ObjectType{AttrTypes: byocSetupAttrTypes()})
		return
	}

	obj, d := types.ObjectValue(byocSetupAttrTypes(), map[string]attr.Value{
		"token":                    stringOrNull(setup.Token),
		"shell_command":            stringOrNull(setup.ShellCommand),
		"shell_command_for_new_vpc": stringOrNull(setup.ShellCommandForNewVpc),
		"url":                      stringOrNull(setup.URL),
		"doc_url":                  stringOrNull(setup.DocURL),
		"url_for_new_vpc":          stringOrNull(setup.URLForNewVpc),
		"doc_url_for_new_vpc":      stringOrNull(setup.DocURLForNewVpc),
	})
	diags.Append(d...)

	list, d := types.ListValue(types.ObjectType{AttrTypes: byocSetupAttrTypes()}, []attr.Value{obj})
	diags.Append(d...)
	state.ByocSetup = list
}

func byocSetupAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"token":                    types.StringType,
		"shell_command":            types.StringType,
		"shell_command_for_new_vpc": types.StringType,
		"url":                      types.StringType,
		"doc_url":                  types.StringType,
		"url_for_new_vpc":          types.StringType,
		"doc_url_for_new_vpc":      types.StringType,
	}
}

// --- Type conversion helpers ---

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func setOptionalString(target **string, val types.String) {
	if !val.IsNull() && !val.IsUnknown() {
		s := val.ValueString()
		*target = &s
	}
}

func setOptionalInt64(target **int64, val types.Int64) {
	if !val.IsNull() && !val.IsUnknown() {
		i := val.ValueInt64()
		*target = &i
	}
}

func setOptionalIntFromInt64(target **int, val types.Int64) {
	if !val.IsNull() && !val.IsUnknown() {
		i := int(val.ValueInt64())
		*target = &i
	}
}
