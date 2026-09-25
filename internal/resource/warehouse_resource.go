package resource

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	_ resource.Resource                   = &WarehouseResource{}
	_ resource.ResourceWithImportState    = &WarehouseResource{}
	_ resource.ResourceWithModifyPlan     = &WarehouseResource{}
	_ resource.ResourceWithValidateConfig = &WarehouseResource{}
)

type WarehouseResource struct {
	client *client.FormationClient
}

func NewWarehouseResource() resource.Resource {
	return &WarehouseResource{}
}

// --- Terraform model ---

type WarehouseResourceModel struct {
	ID                      types.String   `tfsdk:"id"`
	Name                    types.String   `tfsdk:"name"`
	DeploymentMode          types.String   `tfsdk:"deployment_mode"`
	CloudProvider           types.String   `tfsdk:"cloud_provider"`
	Region                  types.String   `tfsdk:"region"`
	SetupMode               types.String   `tfsdk:"setup_mode"`
	VpcMode                 types.String   `tfsdk:"vpc_mode"`
	VpcID                   types.String   `tfsdk:"vpc_id"`
	CredentialID            types.Int64    `tfsdk:"credential_id"`
	NetworkConfigID         types.Int64    `tfsdk:"network_config_id"`
	BucketName              types.String   `tfsdk:"bucket_name"`
	DataCredentialArn       types.String   `tfsdk:"data_credential_arn"`
	DeploymentCredentialArn types.String   `tfsdk:"deployment_credential_arn"`
	SubnetID                types.String   `tfsdk:"subnet_id"`
	SecurityGroupID         types.String   `tfsdk:"security_group_id"`
	EndpointID              types.String   `tfsdk:"endpoint_id"`
	CoreVersion             types.String   `tfsdk:"core_version"`
	CoreVersionID           types.Int64    `tfsdk:"core_version_id"`
	EndpointServiceID       types.String   `tfsdk:"endpoint_service_id"`
	EndpointServiceName     types.String   `tfsdk:"endpoint_service_name"`
	AdminPassword           types.String   `tfsdk:"admin_password"`
	AdminPasswordVersion    types.Int64    `tfsdk:"admin_password_version"`
	Version                 types.String   `tfsdk:"initial_core_version"`
	Tags                    types.Map      `tfsdk:"tags"`
	InitialCluster          types.List     `tfsdk:"initial_cluster"`
	AccessPolicy            types.List     `tfsdk:"public_access_policy"`
	Timeouts                timeouts.Value `tfsdk:"timeouts"`
	// Computed
	Status             types.String `tfsdk:"status"`
	Zone               types.String `tfsdk:"zone"`
	PayType            types.String `tfsdk:"pay_type"`
	CreatedAt          types.String `tfsdk:"created_at"`
	ExpireTime         types.String `tfsdk:"expire_time"`
	ByocSetup          types.List   `tfsdk:"byoc_setup"`
	InitialClusterID   types.String `tfsdk:"initial_cluster_id"`
	TdeEncryptionKeyId types.Int64  `tfsdk:"tde_encryption_key_id"`
	EbsEncryptionKeyId types.Int64  `tfsdk:"ebs_encryption_key_id"`
	EnableTls          types.Bool   `tfsdk:"enable_tls"`
	EnableHttps        types.Bool   `tfsdk:"enable_https"`
}

type InitialClusterModel struct {
	Zone        types.String `tfsdk:"zone"`
	ComputeVcpu types.Int64  `tfsdk:"compute_vcpu"`
	Ratio       types.Int64  `tfsdk:"ratio"`
	CacheGb     types.Int64  `tfsdk:"cache_gb"`
	AutoPause   types.List   `tfsdk:"auto_pause"`
}

type AutoPauseModel struct {
	Enabled            types.Bool  `tfsdk:"enabled"`
	IdleTimeoutMinutes types.Int64 `tfsdk:"idle_timeout_minutes"`
}

type WarehouseAccessPolicyModel struct {
	Policy types.String `tfsdk:"policy"`
	Rules  types.Set    `tfsdk:"rules"`
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
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 32),
					stringvalidator.RegexMatches(regexp.MustCompile(`^[A-Za-z0-9_-]+$`), "must contain only letters, numbers, underscores, and hyphens"),
				},
			},
			"deployment_mode": schema.StringAttribute{
				Description: "Deployment mode: SaaS or BYOC.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("SaaS", "BYOC"),
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
				Validators: []validator.String{
					stringvalidator.OneOf("guided", "advanced"),
				},
			},
			"vpc_mode": schema.StringAttribute{
				Description: "VPC mode hint: existing or new.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("existing", "new"),
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
				Description: "Registered credential configuration ID for advanced AWS BYOC.",
				Optional:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"network_config_id": schema.Int64Attribute{
				Description: "Registered network configuration ID for advanced AWS BYOC.",
				Optional:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
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
				Description: "Current human-readable core version (e.g. 26.1.0). Read-only.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"core_version_id": schema.Int64Attribute{
				Description: "Target core version ID. Changing triggers an upgrade. Discover valid IDs via the velodb_warehouse_versions data source. The API does not return this value on Read, so the resource preserves whatever was last applied (or null if never set).",
				Optional:    true,
			},
			"initial_core_version": schema.StringAttribute{
				Description: "Initial core version to provision, in `major.minor` numeric format (e.g. `26.1`). The management API selects the newest matching build for that major.minor line. Create-only: the API does not return it and rejects changes after creation. To upgrade an existing warehouse, set core_version_id instead.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[0-9]+\.[0-9]+$`),
						"must use major.minor numeric format (e.g. 26.1)",
					),
				},
			},
			"admin_password": schema.StringAttribute{
				Description: "Administrator password. Write-only in the API and preserved as sensitive Terraform state so password rotation can be detected.",
				Optional:    true,
				Sensitive:   true,
			},
			"admin_password_version": schema.Int64Attribute{
				Description: "Increment to trigger a password change.",
				Optional:    true,
			},
			"tags": schema.MapAttribute{
				Description: "Warehouse tags as key/value pairs. Create-only: the management API accepts tags only at creation and does not return or update them, so changing tags after creation is rejected.",
				Optional:    true,
				ElementType: types.StringType,
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
			"endpoint_service_id": schema.StringAttribute{
				Description: "PrivateLink endpoint service ID associated with the warehouse when available.",
				Computed:    true,
			},
			"endpoint_service_name": schema.StringAttribute{
				Description: "PrivateLink endpoint service name associated with the warehouse when available.",
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
			"tde_encryption_key_id": schema.Int64Attribute{
				Description: "TDE encryption key ID.",
				Optional:    true,
			},
			"ebs_encryption_key_id": schema.Int64Attribute{
				Description: "EBS encryption key ID.",
				Optional:    true,
			},
			"enable_tls": schema.BoolAttribute{
				Description: "MySQL/JDBC TLS switch",
				Optional:    true,
			},
			"enable_https": schema.BoolAttribute{
				Description: "HTTP webserver HTTPS switch",
				Optional:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"initial_cluster": schema.ListNestedBlock{
				Description: "Initial cluster created with the warehouse. Create-only.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"zone": schema.StringAttribute{
							Description: "Availability zone.",
							Required:    true,
						},
						"compute_vcpu": schema.Int64Attribute{
							Description: "Compute capacity in vCPUs.",
							Required:    true,
							Validators: []validator.Int64{
								int64validator.AtLeast(4),
							},
						},
						"ratio": schema.Int64Attribute{
							Description: "vCPU-to-memory ratio (1:4 or 1:8). Create-only; omit to use the Manager default (1:8).",
							Optional:    true,
							Validators: []validator.Int64{
								int64validator.OneOf(4, 8),
							},
						},
						"cache_gb": schema.Int64Attribute{
							Description: "Cache capacity in GB.",
							Required:    true,
							Validators: []validator.Int64{
								int64validator.AtLeast(100),
							},
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
										Description: "Idle timeout in minutes. Required when enabled is true.",
										Optional:    true,
										Validators: []validator.Int64{
											int64validator.AtLeast(0),
										},
									},
								},
							},
						},
					},
				},
			},
			"public_access_policy": schema.ListNestedBlock{
				Description: "Initial public access policy applied at warehouse creation. BYOC only and create-only: the policy is set once during provisioning. Manage it afterward with the velodb_warehouse_public_access_policy resource.",
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"policy": schema.StringAttribute{
							Description: "Public access policy: DENY_ALL, ALLOW_ALL, or ALLOWLIST_ONLY.",
							Required:    true,
							Validators: []validator.String{
								stringvalidator.OneOf("DENY_ALL", "ALLOW_ALL", "ALLOWLIST_ONLY"),
							},
						},
						"rules": schema.SetNestedAttribute{
							Description: "Allowlist CIDR rules. Only valid when policy is ALLOWLIST_ONLY. Order is not significant.",
							Optional:    true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"cidr": schema.StringAttribute{
										Description: "CIDR block or single IP.",
										Required:    true,
									},
									"description": schema.StringAttribute{
										Description: "Optional rule description.",
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
						"token":                     schema.StringAttribute{Computed: true},
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

func (r *WarehouseResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	// initial_core_version (initial core version) and core_version_id (explicit
	// upgrade target) are mutually exclusive: setting both provisions at
	// initial_core_version and then immediately upgrades, a redundant double operation.
	var version types.String
	var coreVersionID types.Int64
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("initial_core_version"), &version)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("core_version_id"), &coreVersionID)...)
	if !version.IsNull() && !version.IsUnknown() && !coreVersionID.IsNull() && !coreVersionID.IsUnknown() {
		resp.Diagnostics.AddError(
			"initial_core_version and core_version_id cannot be set together",
			"Set initial_core_version to pin the core version at creation, or core_version_id to upgrade an existing "+
				"warehouse — not both. Setting both provisions at initial_core_version and then upgrades in the same apply.",
		)
	}

	// Validate public_access_policy first: it is independent of initial_cluster,
	// which short-circuits validation below when unknown.
	var accessPolicy types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("public_access_policy"), &accessPolicy)...)
	if !accessPolicy.IsNull() && !accessPolicy.IsUnknown() && len(accessPolicy.Elements()) > 0 {
		var policies []WarehouseAccessPolicyModel
		resp.Diagnostics.Append(accessPolicy.ElementsAs(ctx, &policies, false)...)
		if !resp.Diagnostics.HasError() {
			validatePublicAccessPolicy(&resp.Diagnostics, policies[0].Policy, policies[0].Rules)
		}
	}

	var initialCluster types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("initial_cluster"), &initialCluster)...)
	if initialCluster.IsUnknown() {
		return
	}
	if !initialCluster.IsNull() && len(initialCluster.Elements()) > 0 {
		var clusters []InitialClusterModel
		resp.Diagnostics.Append(initialCluster.ElementsAs(ctx, &clusters, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		validateAutoPauseRequiresTimeout(ctx, &resp.Diagnostics, "initial_cluster", clusters[0].AutoPause)
	}

	rejectUnsupportedString(ctx, req, resp, "vpc_id")
	rejectUnsupportedString(ctx, req, resp, "bucket_name")
	rejectUnsupportedString(ctx, req, resp, "data_credential_arn")
	rejectUnsupportedString(ctx, req, resp, "deployment_credential_arn")
	rejectUnsupportedString(ctx, req, resp, "subnet_id")
	rejectUnsupportedString(ctx, req, resp, "security_group_id")
	rejectUnsupportedString(ctx, req, resp, "endpoint_id")
}

func (r *WarehouseResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Nothing to enforce on destroy.
	if req.Plan.Raw.IsNull() {
		return
	}

	// Update path: reject changes to create-only attributes.
	if !req.State.Raw.IsNull() {
		// When another attribute already forces replacement, the resource is
		// destroyed and recreated, so create-only values legitimately apply to
		// the new instance — skip the change guards.
		if len(resp.RequiresReplace) > 0 {
			return
		}

		var plan, state WarehouseResourceModel
		resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}
		rejectCreateOnlyChange(&resp.Diagnostics, path.Root("tags"), plan.Tags, state.Tags,
			"Warehouse tags cannot be changed after creation",
			"The VeloDB management API accepts warehouse tags only at creation time and cannot update them. "+
				"Revert the tags change, or destroy and recreate the warehouse to apply new tags.")
		rejectCreateOnlyChange(&resp.Diagnostics, path.Root("initial_core_version"), plan.Version, state.Version,
			"Warehouse initial_core_version cannot be changed after creation",
			"The VeloDB management API accepts initial_core_version only when creating a warehouse. "+
				"Revert the initial_core_version change and set core_version_id to upgrade an existing warehouse "+
				"(discover valid IDs via the velodb_warehouse_versions data source).")
		rejectCreateOnlyChange(&resp.Diagnostics, path.Root("public_access_policy"), plan.AccessPolicy, state.AccessPolicy,
			"Warehouse public_access_policy cannot be changed after creation",
			"public_access_policy sets the initial public access policy only at creation time. "+
				"Revert the change and manage the policy after creation with the "+
				"velodb_warehouse_public_access_policy resource.")
		return
	}

	// Create path.
	if !req.Plan.Raw.IsKnown() {
		return
	}
	var plan WarehouseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateWarehouseCreation(&resp.Diagnostics, &plan, true)
}

// rejectCreateOnlyChange errors when a create-only attribute is changed after
// creation. It skips when the planned value is unknown (interpolated — it may
// resolve to the existing value) or when the prior state has no value (e.g.
// after import, where create-only fields are never read back and would
// otherwise block every apply).
func rejectCreateOnlyChange(diags *diag.Diagnostics, p path.Path, planVal, stateVal attr.Value, summary, detail string) {
	if planVal.IsUnknown() || stateVal.IsNull() {
		return
	}
	if !planVal.Equal(stateVal) {
		diags.AddAttributeError(p, summary, detail)
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

	validateWarehouseCreation(&resp.Diagnostics, &plan, false)
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
		DeploymentMode: normalizeDeploymentMode(plan.DeploymentMode.ValueString()),
		CloudProvider:  plan.CloudProvider.ValueString(),
		Region:         plan.Region.ValueString(),
	}
	var tags map[string]string
	plan.Tags.ElementsAs(ctx, &tags, false)
	createReq.Tags = tags
	setOptionalString(&createReq.Version, plan.Version)
	setOptionalString(&createReq.VpcMode, plan.VpcMode)
	setOptionalString(&createReq.SetupMode, plan.SetupMode)
	// Initial access policy (BYOC only, create-only).
	createReq.AccessPolicy = warehouseAccessPolicyRequest(ctx, plan.AccessPolicy, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	setOptionalInt64(&createReq.CredentialID, plan.CredentialID)
	setOptionalInt64(&createReq.NetworkConfigID, plan.NetworkConfigID)
	setOptionalString(&createReq.AdminPassword, plan.AdminPassword)
	setOptionalInt64(&createReq.TdeEncryptionKeyId, plan.TdeEncryptionKeyId)
	setOptionalInt64(&createReq.EbsEncryptionKeyId, plan.EbsEncryptionKeyId)
	setOptionalBool(&createReq.EnableTls, plan.EnableTls)
	setOptionalBool(&createReq.EnableHttps, plan.EnableHttps)
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
				Zone:        ic.Zone.ValueString(),
				ComputeVcpu: int(ic.ComputeVcpu.ValueInt64()),
				CacheGb:     int(ic.CacheGb.ValueInt64()),
			}
			setOptionalIntFromInt64(&clReq.Ratio, ic.Ratio)

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
		resp.Diagnostics.AddError(userError("creating warehouse", err))
		return
	}

	plan.ID = types.StringValue(result.WarehouseID)

	// Store BYOC setup if returned
	r.setByocSetup(ctx, &plan, result.SetupGuide, &resp.Diagnostics)

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

	if !plan.CoreVersionID.IsNull() && !plan.CoreVersionID.IsUnknown() {
		if plan.CoreVersionID.ValueInt64() <= 0 {
			resp.Diagnostics.AddError(
				"Invalid core_version_id",
				"core_version_id must be a positive core version ID.",
			)
			return
		}
		if err := r.client.UpgradeWarehouse(ctx, result.WarehouseID, plan.CoreVersionID.ValueInt64()); err != nil {
			resp.Diagnostics.AddError(userError("upgrading warehouse", err))
			return
		}
		_, err = client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
			wh, err := r.client.GetWarehouse(ctx, result.WarehouseID)
			if err != nil {
				return "", err
			}
			return wh.Status, nil
		}, []string{"Running"}, client.FailedStatuses, createTimeout, 15*time.Second)
		if err != nil {
			resp.Diagnostics.AddWarning("Warehouse upgrade may still be in progress", err.Error())
		}
	}

	// Read back state
	r.readWarehouseIntoState(ctx, result.WarehouseID, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

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
	validateInitialClusterCreateOnlyRatio(ctx, &resp.Diagnostics, plan.InitialCluster, state.InitialCluster)
	if resp.Diagnostics.HasError() {
		return
	}

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

	// Trigger version upgrade if core_version_id changed.
	// Guard against zero IDs (which the velodb_warehouse_versions data source returns
	// when the API has no available versions) — they would always 409 "targetVersionId not found".
	if !plan.CoreVersionID.IsNull() && !plan.CoreVersionID.IsUnknown() &&
		!plan.CoreVersionID.Equal(state.CoreVersionID) {
		if plan.CoreVersionID.ValueInt64() <= 0 {
			resp.Diagnostics.AddError(
				"Invalid core_version_id",
				"core_version_id must be a positive core version ID. "+
					"This is typically caused by referencing default_id from velodb_warehouse_versions when the API returned no available versions. "+
					"Either remove core_version_id from the configuration or pin a specific version_id.",
			)
			return
		}
		if err := r.client.UpgradeWarehouse(ctx, warehouseID, plan.CoreVersionID.ValueInt64()); err != nil {
			resp.Diagnostics.AddError(userError("upgrading warehouse", err))
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
	if resp.Diagnostics.HasError() {
		return
	}
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
		if !warehouseDeleteInProgress(err) {
			resp.Diagnostics.AddError(userError("deleting warehouse", err))
			return
		}
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
		resp.Diagnostics.AddError("Warehouse deletion did not complete", err.Error())
	}
}

func warehouseDeleteInProgress(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) &&
		apiErr.Code == "OperationConflict" &&
		strings.Contains(strings.ToLower(apiErr.Message), "already in deleting")
}

func (r *WarehouseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	warehouseID := strings.TrimSpace(req.ID)
	if warehouseID == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected warehouse_id.")
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The VeloDB client is not available during import.")
		return
	}

	if _, err := r.client.GetWarehouse(ctx, warehouseID); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.Diagnostics.AddError(
				"Warehouse not found",
				fmt.Sprintf("Warehouse %q does not exist or is not accessible. Verify the warehouse_id before importing.", warehouseID),
			)
			return
		}
		resp.Diagnostics.AddError(userError("importing warehouse", err))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), warehouseID)...)
}

// --- Helpers ---

func (r *WarehouseResource) readWarehouseIntoState(ctx context.Context, warehouseID string, state *WarehouseResourceModel, diags *diag.Diagnostics) {
	wh, err := r.client.GetWarehouse(ctx, warehouseID)
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			state.ID = types.StringNull()
			return
		}
		diags.AddError(userError("reading warehouse", err))
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
	state.EndpointServiceID = stringOrNull(wh.EndpointServiceID)
	state.EndpointServiceName = stringOrNull(wh.EndpointServiceName)

	if wh.CreatedAt != nil {
		state.CreatedAt = types.StringValue(wh.CreatedAt.Format(time.RFC3339))
	}
	if wh.ExpireTime != nil {
		state.ExpireTime = types.StringValue(wh.ExpireTime.Format(time.RFC3339))
	} else {
		state.ExpireTime = types.StringNull()
	}

	// Find initial_cluster ID by selecting the first non-system cluster.
	if state.InitialClusterID.IsNull() || state.InitialClusterID.IsUnknown() {
		clusters, err := r.client.ListClusters(ctx, warehouseID, &client.ListClustersOptions{Page: 1, Size: 50})
		if err == nil {
			for _, c := range clusters.Data {
				if strings.HasPrefix(c.ClusterID, "m-") {
					continue
				}
				state.InitialClusterID = types.StringValue(c.ClusterID)
				break
			}
		}
		if state.InitialClusterID.IsNull() || state.InitialClusterID.IsUnknown() {
			state.InitialClusterID = types.StringNull()
		}
	}

	if wh.SetupGuide != nil {
		r.setByocSetup(ctx, state, wh.SetupGuide, diags)
	}
}

func (r *WarehouseResource) setByocSetup(ctx context.Context, state *WarehouseResourceModel, setup *client.WarehouseSetupGuide, diags *diag.Diagnostics) {
	if setup == nil {
		state.ByocSetup = types.ListNull(types.ObjectType{AttrTypes: byocSetupAttrTypes()})
		return
	}

	obj, d := types.ObjectValue(byocSetupAttrTypes(), map[string]attr.Value{
		"token":                     types.StringNull(),
		"shell_command":             stringOrNull(setup.ShellCommand),
		"shell_command_for_new_vpc": types.StringNull(),
		"url":                       stringOrNull(setup.SetupURL),
		"doc_url":                   stringOrNull(setup.GuideURL),
		"url_for_new_vpc":           types.StringNull(),
		"doc_url_for_new_vpc":       types.StringNull(),
	})
	diags.Append(d...)

	list, d := types.ListValue(types.ObjectType{AttrTypes: byocSetupAttrTypes()}, []attr.Value{obj})
	diags.Append(d...)
	state.ByocSetup = list
}

func byocSetupAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"token":                     types.StringType,
		"shell_command":             types.StringType,
		"shell_command_for_new_vpc": types.StringType,
		"url":                       types.StringType,
		"doc_url":                   types.StringType,
		"url_for_new_vpc":           types.StringType,
		"doc_url_for_new_vpc":       types.StringType,
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

func setOptionalBool(target **bool, val types.Bool) {
	if !val.IsNull() && !val.IsUnknown() {
		i := val.ValueBool()
		*target = &i
	}
}
