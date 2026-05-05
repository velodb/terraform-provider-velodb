package resource

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

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
	SetupMode               types.String  `tfsdk:"setup_mode"`
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
	CoreVersionID            types.Int64   `tfsdk:"core_version_id"`
	AdminPassword            types.String  `tfsdk:"admin_password"`
	AdminPasswordVersion     types.Int64   `tfsdk:"admin_password_version"`
	UpgradePolicy            types.String  `tfsdk:"upgrade_policy"`
	MaintenanceWindow        types.Object  `tfsdk:"maintenance_window"`
	Tags                     types.Map     `tfsdk:"tags"`
	InitialCluster           types.List    `tfsdk:"initial_cluster"`
	Timeouts                 timeouts.Value `tfsdk:"timeouts"`
	// Computed
	Status           types.String `tfsdk:"status"`
	Zone             types.String `tfsdk:"zone"`
	PayType          types.String `tfsdk:"pay_type"`
	CreatedAt        types.String `tfsdk:"created_at"`
	ExpireTime       types.String `tfsdk:"expire_time"`
	ByocSetup        types.List   `tfsdk:"byoc_setup"`
	InitialClusterID types.String `tfsdk:"initial_cluster_id"`
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

type MaintenanceWindowModel struct {
	StartHourUtc types.Int64 `tfsdk:"start_hour_utc"`
	EndHourUtc   types.Int64 `tfsdk:"end_hour_utc"`
}

func maintenanceWindowAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"start_hour_utc": types.Int64Type,
		"end_hour_utc":   types.Int64Type,
	}
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
			"setup_mode": schema.StringAttribute{
				Description: "BYOC setup mode: `guided` (CloudFormation template) or `advanced` (pre-existing AWS resources — IaC-friendly).",
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
				Description: "Current human-readable engine version (e.g. 3.0.8). Read-only.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"core_version_id": schema.Int64Attribute{
				Description: "Target engine version ID. Changing triggers an upgrade. Discover valid IDs via the velodb_warehouse_versions data source. The API does not return this value on Read, so the resource preserves whatever was last applied (or null if never set).",
				Optional:    true,
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
			"upgrade_policy": schema.StringAttribute{
				Description: "Upgrade policy for the warehouse (e.g. \"automatic\"). Once set, removing from configuration retains the API value (the API does not support clearing it).",
				Optional:    true,
				Computed:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"maintenance_window": schema.SingleNestedAttribute{
				Description: "Maintenance window for automatic upgrades. Hours are in UTC, 0-23. Once set, removing from configuration retains the API value (the API does not support clearing it).",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"start_hour_utc": schema.Int64Attribute{
						Description: "Maintenance window start hour in UTC (0-23).",
						Required:    true,
						Validators:  []validator.Int64{int64validator.Between(0, 23)},
					},
					"end_hour_utc": schema.Int64Attribute{
						Description: "Maintenance window end hour in UTC (0-23).",
						Required:    true,
						Validators:  []validator.Int64{int64validator.Between(0, 23)},
					},
				},
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zone": schema.StringAttribute{
				Description: "Primary availability zone.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"pay_type": schema.StringAttribute{
				Description: "Billing type (PostPaid or PrePaid).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"initial_cluster_id": schema.StringAttribute{
				Description: "ID of the initial cluster created with the warehouse. Use this to import the cluster as a velodb_cluster resource (e.g., to delete it after you add other clusters).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
	setOptionalString(&createReq.SetupMode, plan.SetupMode)
	setOptionalString(&createReq.VpcID, plan.VpcID)
	setOptionalInt64(&createReq.CredentialID, plan.CredentialID)
	setOptionalInt64(&createReq.NetworkConfigID, plan.NetworkConfigID)
	setOptionalString(&createReq.BucketName, plan.BucketName)
	setOptionalString(&createReq.DataCredentialArn, plan.DataCredentialArn)
	setOptionalString(&createReq.DeploymentCredentialArn, plan.DeploymentCredentialArn)
	setOptionalString(&createReq.SubnetID, plan.SubnetID)
	setOptionalString(&createReq.SecurityGroupID, plan.SecurityGroupID)
	setOptionalString(&createReq.EndpointID, plan.EndpointID)
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

	// CreateWarehouseRequest has no upgradePolicy/maintenanceWindow fields, so apply
	// the user's settings via PATCH /settings now that the warehouse exists.
	if (!plan.UpgradePolicy.IsNull() && !plan.UpgradePolicy.IsUnknown()) ||
		(!plan.MaintenanceWindow.IsNull() && !plan.MaintenanceWindow.IsUnknown()) {
		settingsReq := &client.UpdateWarehouseSettingsRequest{}
		if !plan.UpgradePolicy.IsNull() && !plan.UpgradePolicy.IsUnknown() {
			s := plan.UpgradePolicy.ValueString()
			settingsReq.UpgradePolicy = &s
		}
		if !plan.MaintenanceWindow.IsNull() && !plan.MaintenanceWindow.IsUnknown() {
			var mw MaintenanceWindowModel
			resp.Diagnostics.Append(plan.MaintenanceWindow.As(ctx, &mw, basetypes.ObjectAsOptions{})...)
			if resp.Diagnostics.HasError() {
				return
			}
			settingsReq.MaintenanceWindow = &client.MaintenanceWindow{
				StartHourUtc: int(mw.StartHourUtc.ValueInt64()),
				EndHourUtc:   int(mw.EndHourUtc.ValueInt64()),
			}
		}
		if err := r.client.UpdateWarehouseSettings(ctx, result.WarehouseID, settingsReq); err != nil {
			resp.Diagnostics.AddWarning("Warehouse settings not applied at create", err.Error())
		}
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
	priorTimeouts := state.Timeouts

	r.readWarehouseIntoState(ctx, state.ID.ValueString(), &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	state.AdminPassword = priorPassword
	state.AdminPasswordVersion = priorPasswordVersion
	state.InitialCluster = priorInitialCluster
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

	// Rename via PATCH /warehouses/{id}
	if !plan.Name.Equal(state.Name) {
		updateReq := &client.UpdateWarehouseRequest{}
		s := plan.Name.ValueString()
		updateReq.Name = &s
		if err := r.client.UpdateWarehouse(ctx, warehouseID, updateReq); err != nil {
			resp.Diagnostics.AddError("Error renaming warehouse", err.Error())
			return
		}
	}

	// Update upgrade policy / maintenance window via PATCH /warehouses/{id}/settings.
	// API requires at least one of upgradePolicy or maintenanceWindow — skip the call
	// entirely if both ended up null in the plan, which would otherwise produce a 400.
	if !plan.UpgradePolicy.Equal(state.UpgradePolicy) || !plan.MaintenanceWindow.Equal(state.MaintenanceWindow) {
		settingsReq := &client.UpdateWarehouseSettingsRequest{}
		if !plan.UpgradePolicy.IsNull() && !plan.UpgradePolicy.IsUnknown() {
			s := plan.UpgradePolicy.ValueString()
			settingsReq.UpgradePolicy = &s
		}
		if !plan.MaintenanceWindow.IsNull() && !plan.MaintenanceWindow.IsUnknown() {
			var mw MaintenanceWindowModel
			resp.Diagnostics.Append(plan.MaintenanceWindow.As(ctx, &mw, basetypes.ObjectAsOptions{})...)
			if resp.Diagnostics.HasError() {
				return
			}
			settingsReq.MaintenanceWindow = &client.MaintenanceWindow{
				StartHourUtc: int(mw.StartHourUtc.ValueInt64()),
				EndHourUtc:   int(mw.EndHourUtc.ValueInt64()),
			}
		}
		if settingsReq.UpgradePolicy == nil && settingsReq.MaintenanceWindow == nil {
			resp.Diagnostics.AddWarning(
				"Cannot clear both upgrade_policy and maintenance_window",
				"The Management API requires at least one of upgrade_policy or maintenance_window to be set when calling PATCH /settings. "+
					"Removing both from configuration would produce a 400 error, so this update was skipped. "+
					"To change settings, keep at least one of the two fields populated.",
			)
		} else if err := r.client.UpdateWarehouseSettings(ctx, warehouseID, settingsReq); err != nil {
			resp.Diagnostics.AddError("Error updating warehouse settings", err.Error())
			return
		}
	}

	// Trigger version upgrade if core_version_id changed.
	// Guard against zero IDs (which the velodb_warehouse_versions data source returns
	// when the API has no available versions) — they would always 409 "targetVersionId not found".
	if !plan.CoreVersionID.IsNull() && !plan.CoreVersionID.IsUnknown() &&
		!plan.CoreVersionID.Equal(state.CoreVersionID) {
		if plan.CoreVersionID.ValueInt64() <= 0 {
			resp.Diagnostics.AddError(
				"Invalid core_version_id",
				"core_version_id must be a positive engine version ID. "+
					"This is typically caused by referencing default_id from velodb_warehouse_versions when the API returned no available versions. "+
					"Either remove core_version_id from the configuration or pin a specific version_id.",
			)
			return
		}
		if err := r.client.UpgradeWarehouse(ctx, warehouseID, plan.CoreVersionID.ValueInt64()); err != nil {
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

	// Settings (upgrade policy + maintenance window)
	if settings, err := r.client.GetWarehouseSettings(ctx, warehouseID); err == nil && settings != nil {
		state.UpgradePolicy = stringOrNull(settings.UpgradePolicy)
		if settings.MaintenanceWindow != nil {
			obj, d := types.ObjectValue(maintenanceWindowAttrTypes(), map[string]attr.Value{
				"start_hour_utc": types.Int64Value(int64(settings.MaintenanceWindow.StartHourUtc)),
				"end_hour_utc":   types.Int64Value(int64(settings.MaintenanceWindow.EndHourUtc)),
			})
			diags.Append(d...)
			state.MaintenanceWindow = obj
		} else {
			state.MaintenanceWindow = types.ObjectNull(maintenanceWindowAttrTypes())
		}
	} else {
		state.UpgradePolicy = types.StringNull()
		state.MaintenanceWindow = types.ObjectNull(maintenanceWindowAttrTypes())
	}

	// Find initial_cluster ID by listing clusters and matching the configured name
	if state.InitialClusterID.IsNull() || state.InitialClusterID.IsUnknown() {
		initialName := ""
		if !state.InitialCluster.IsNull() && !state.InitialCluster.IsUnknown() {
			var ics []InitialClusterModel
			state.InitialCluster.ElementsAs(ctx, &ics, false)
			if len(ics) > 0 {
				initialName = ics[0].Name.ValueString()
			}
		}
		if initialName == "" {
			initialName = "initial_cluster" // default name set by API
		}
		clusters, err := r.client.ListClusters(ctx, warehouseID, &client.ListClustersOptions{Page: 1, Size: 50})
		if err == nil {
			for _, c := range clusters.Data {
				if strings.HasPrefix(c.ClusterID, "m-") {
					continue
				}
				if c.Name == initialName {
					state.InitialClusterID = types.StringValue(c.ClusterID)
					break
				}
			}
		}
		if state.InitialClusterID.IsNull() || state.InitialClusterID.IsUnknown() {
			state.InitialClusterID = types.StringNull()
		}
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
