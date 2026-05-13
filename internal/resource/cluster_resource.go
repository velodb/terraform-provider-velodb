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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

var (
	_ resource.Resource                   = &ClusterResource{}
	_ resource.ResourceWithImportState    = &ClusterResource{}
	_ resource.ResourceWithValidateConfig = &ClusterResource{}
)

type ClusterResource struct {
	client *client.FormationClient
}

func NewClusterResource() resource.Resource {
	return &ClusterResource{}
}

// --- Terraform models ---

type ClusterResourceModel struct {
	ID            types.String   `tfsdk:"id"`
	WarehouseID   types.String   `tfsdk:"warehouse_id"`
	Name          types.String   `tfsdk:"name"`
	ClusterType   types.String   `tfsdk:"cluster_type"`
	Zone          types.String   `tfsdk:"zone"`
	ComputeVcpu   types.Int64    `tfsdk:"compute_vcpu"`
	CacheGb       types.Int64    `tfsdk:"cache_gb"`
	BillingMethod types.String   `tfsdk:"billing_method"`
	DesiredState  types.String   `tfsdk:"desired_state"`
	RebootTrigger types.Int64    `tfsdk:"reboot_trigger"`
	AutoPause     types.List     `tfsdk:"auto_pause"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
	// Computed
	Status         types.String `tfsdk:"status"`
	CloudProvider  types.String `tfsdk:"cloud_provider"`
	Region         types.String `tfsdk:"region"`
	TotalCpu       types.Int64  `tfsdk:"total_cpu"`
	TotalDiskGb    types.Int64  `tfsdk:"total_disk_gb"`
	NodeCount      types.Int64  `tfsdk:"node_count"`
	CreatedAt      types.String `tfsdk:"created_at"`
	StartedAt      types.String `tfsdk:"started_at"`
	ExpireTime     types.String `tfsdk:"expire_time"`
	ConnectionInfo types.List   `tfsdk:"connection_info"`
}

type ConnectionInfoModel struct {
	PublicEndpoint  types.String `tfsdk:"public_endpoint"`
	PrivateEndpoint types.String `tfsdk:"private_endpoint"`
	ListenerPort    types.Int64  `tfsdk:"listener_port"`
}

// --- Metadata / Schema ---

func (r *ClusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (r *ClusterResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a VeloDB Cloud cluster within a warehouse.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Cluster identifier.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"warehouse_id": schema.StringAttribute{
				Description: "Parent warehouse identifier.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Cluster display name.",
				Required:    true,
			},
			"cluster_type": schema.StringAttribute{
				Description: "Cluster type. Only COMPUTE is supported.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("COMPUTE"),
				},
			},
			"zone": schema.StringAttribute{
				Description: "Availability zone.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"compute_vcpu": schema.Int64Attribute{
				Description: "vCPU capacity: 4, 8, 16, or multiples of 16.",
				Required:    true,
				Validators:  []validator.Int64{int64validator.AtLeast(4)},
			},
			"cache_gb": schema.Int64Attribute{
				Description: "Cache disk size in GB (minimum 100).",
				Required:    true,
				Validators:  []validator.Int64{int64validator.AtLeast(100)},
			},
			"billing_method": schema.StringAttribute{
				Description: "Billing method: on_demand.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"desired_state": schema.StringAttribute{
				Description: "Desired cluster state: running or paused.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("running", "paused"),
				},
			},
			"reboot_trigger": schema.Int64Attribute{
				Description: "Increment to trigger a /reboot action.",
				Optional:    true,
			},
			// Computed
			"status": schema.StringAttribute{
				Description: "Observed cluster status.",
				Computed:    true,
			},
			"cloud_provider": schema.StringAttribute{Computed: true, Description: "Cloud provider."},
			"region":         schema.StringAttribute{Computed: true, Description: "Cloud region."},
			"total_cpu":      schema.Int64Attribute{Computed: true, Description: "Total CPU."},
			"total_disk_gb":  schema.Int64Attribute{Computed: true, Description: "Total disk GB."},
			"node_count":     schema.Int64Attribute{Computed: true, Description: "Total node count."},
			"created_at": schema.StringAttribute{
				Computed: true, Description: "Creation time in RFC 3339 format.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"started_at":  schema.StringAttribute{Computed: true, Description: "Start time in RFC 3339 format."},
			"expire_time": schema.StringAttribute{Computed: true, Description: "Expiration time when applicable."},
			"connection_info": schema.ListNestedAttribute{
				Description: "Cluster connection endpoints.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"public_endpoint":  schema.StringAttribute{Computed: true},
						"private_endpoint": schema.StringAttribute{Computed: true},
						"listener_port":    schema.Int64Attribute{Computed: true},
					},
				},
			},
		},
		Blocks: map[string]schema.Block{
			"auto_pause": schema.ListNestedBlock{
				Description: "Auto-pause configuration (applies to whole cluster).",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"enabled": schema.BoolAttribute{Required: true, Description: "Whether auto-pause is enabled."},
						"idle_timeout_minutes": schema.Int64Attribute{
							Optional: true, Description: "Idle minutes before auto-pause.",
						},
					},
				},
			},
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *ClusterResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var computeVcpu types.Int64
	var cacheGb types.Int64
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("compute_vcpu"), &computeVcpu)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("cache_gb"), &cacheGb)...)
}

func (r *ClusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 20*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	warehouseID := plan.WarehouseID.ValueString()

	createReq := &client.CreateClusterRequest{
		Name:        plan.Name.ValueString(),
		ClusterType: plan.ClusterType.ValueString(),
		ComputeVcpu: int(plan.ComputeVcpu.ValueInt64()),
		CacheGb:     int(plan.CacheGb.ValueInt64()),
	}
	if !plan.BillingMethod.IsNull() && !plan.BillingMethod.IsUnknown() {
		createReq.BillingModel = stringPtr(plan.BillingMethod.ValueString())
	} else {
		createReq.BillingModel = stringPtr("on_demand")
	}
	setOptionalString(&createReq.Zone, plan.Zone)
	createReq.AutoPause = r.buildAutoPause(ctx, plan.AutoPause, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.CreateCluster(ctx, warehouseID, createReq)
	if err != nil {
		resp.Diagnostics.AddError(userError("creating cluster", err))
		return
	}
	clusterID := result.ClusterID
	plan.ID = types.StringValue(clusterID)

	// Wait to Running
	_, err = client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
		cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
		if err != nil {
			return "", err
		}
		return cl.Status, nil
	}, []string{"Running"}, client.FailedStatuses, createTimeout, 15*time.Second)
	if err != nil {
		resp.Diagnostics.AddWarning("Cluster created but not yet Running", err.Error())
	}

	// Handle initial desired_state = paused
	if !plan.DesiredState.IsNull() && plan.DesiredState.ValueString() == "paused" {
		if err := r.client.OperateCluster(ctx, warehouseID, clusterID, "pause"); err != nil {
			resp.Diagnostics.AddError(userError("pausing cluster after creation", err))
			return
		}
		_, _ = client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
			cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
			if err != nil {
				return "", err
			}
			return cl.Status, nil
		}, []string{"Suspended", "Stopped"}, client.FailedStatuses, createTimeout, 10*time.Second)
	}

	r.readClusterIntoState(ctx, warehouseID, clusterID, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve only what truly can't be read back
	priorAutoPause := state.AutoPause
	priorRebootTrigger := state.RebootTrigger
	priorTimeouts := state.Timeouts

	r.readClusterIntoState(ctx, state.WarehouseID.ValueString(), state.ID.ValueString(), &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	state.AutoPause = priorAutoPause
	state.RebootTrigger = priorRebootTrigger
	state.Timeouts = priorTimeouts

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state ClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	warehouseID := state.WarehouseID.ValueString()
	clusterID := state.ID.ValueString()

	// Block simultaneous CPU and disk changes
	cpuChanged := !plan.ComputeVcpu.Equal(state.ComputeVcpu)
	diskChanged := !plan.CacheGb.Equal(state.CacheGb)
	if cpuChanged && diskChanged {
		resp.Diagnostics.AddError(
			"Simultaneous compute_vcpu and cache_gb changes are not supported",
			"VeloDB does not allow changing both compute_vcpu and cache_gb in the same apply. "+
				"Please apply them in separate steps: change one value, apply, then change the other.",
		)
		return
	}

	// 1. desired_state transitions (pause/resume) — only if cluster isn't already there
	if !plan.DesiredState.Equal(state.DesiredState) {
		action := r.resolveAction(plan.DesiredState.ValueString())
		targetStatuses := desiredStateToStatuses(plan.DesiredState.ValueString())
		if action != "" {
			// Check current status — skip action if already in desired state
			cur, err := r.client.GetCluster(ctx, warehouseID, clusterID)
			if err != nil {
				resp.Diagnostics.AddError(userError("reading cluster before action", err))
				return
			}
			alreadyThere := false
			for _, s := range targetStatuses {
				if cur.Status == s {
					alreadyThere = true
					break
				}
			}
			if !alreadyThere {
				if err := r.client.OperateCluster(ctx, warehouseID, clusterID, action); err != nil {
					resp.Diagnostics.AddError(userError("performing cluster action", err))
					return
				}
				_, err := client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
					cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
					if err != nil {
						return "", err
					}
					return cl.Status, nil
				}, targetStatuses, client.FailedStatuses, updateTimeout, 10*time.Second)
				if err != nil {
					resp.Diagnostics.AddWarning("Cluster action may still be in progress", err.Error())
				}
			}
		}
	}

	// 2. Reboot trigger
	if !plan.RebootTrigger.IsNull() && !plan.RebootTrigger.Equal(state.RebootTrigger) {
		if err := r.client.RebootCluster(ctx, warehouseID, clusterID); err != nil {
			resp.Diagnostics.AddError(userError("rebooting cluster", err))
			return
		}
		_, _ = client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
			cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
			if err != nil {
				return "", err
			}
			return cl.Status, nil
		}, []string{"Running"}, client.FailedStatuses, updateTimeout, 10*time.Second)
	}

	// 3. Resize compute_vcpu
	if !plan.ComputeVcpu.Equal(state.ComputeVcpu) {
		v := int(plan.ComputeVcpu.ValueInt64())
		updateReq := &client.UpdateClusterRequest{
			ComputeVcpu: &v,
		}
		if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, updateReq); err != nil {
			resp.Diagnostics.AddError(userError("resizing compute_vcpu", err))
			return
		}
		r.waitStable(ctx, warehouseID, clusterID, updateTimeout, &resp.Diagnostics)
	}

	// 4. Resize cache_gb
	if !plan.CacheGb.Equal(state.CacheGb) {
		v := int(plan.CacheGb.ValueInt64())
		updateReq := &client.UpdateClusterRequest{
			CacheGb: &v,
		}
		if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, updateReq); err != nil {
			resp.Diagnostics.AddError(userError("resizing cache_gb", err))
			return
		}
		r.waitStable(ctx, warehouseID, clusterID, updateTimeout, &resp.Diagnostics)
	}

	// 5. Name change
	if !plan.Name.Equal(state.Name) {
		s := plan.Name.ValueString()
		if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, &client.UpdateClusterRequest{Name: &s}); err != nil {
			resp.Diagnostics.AddError(userError("updating cluster name", err))
			return
		}
	}

	// 6. auto_pause change
	if !plan.AutoPause.Equal(state.AutoPause) {
		ap := r.buildAutoPause(ctx, plan.AutoPause, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		if ap != nil {
			if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, &client.UpdateClusterRequest{AutoPause: ap}); err != nil {
				resp.Diagnostics.AddError(userError("updating auto_pause", err))
				return
			}
		}
	}

	r.readClusterIntoState(ctx, warehouseID, clusterID, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, 15*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	warehouseID := state.WarehouseID.ValueString()
	clusterID := state.ID.ValueString()

	if err := r.client.DeleteCluster(ctx, warehouseID, clusterID); err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError(userError("deleting cluster", err))
		return
	}

	_, err := client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
		cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
		if err != nil {
			return "", err
		}
		return cl.Status, nil
	}, []string{"Deleted"}, nil, deleteTimeout, 15*time.Second)
	if err != nil {
		resp.Diagnostics.AddWarning("Cluster deletion may still be in progress", err.Error())
	}
}

func (r *ClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: warehouse_id/cluster_id")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("warehouse_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// --- Helpers ---

func (r *ClusterResource) waitStable(ctx context.Context, warehouseID, clusterID string, timeout time.Duration, diags *diag.Diagnostics) {
	_, err := client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
		cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
		if err != nil {
			return "", err
		}
		return cl.Status, nil
	}, client.StableStatuses, client.FailedStatuses, timeout, 15*time.Second)
	if err != nil {
		diags.AddWarning("Cluster operation may still be in progress", err.Error())
	}
}

func (r *ClusterResource) resolveAction(newDesired string) string {
	switch newDesired {
	case "running":
		return "resume"
	case "paused":
		return "pause"
	}
	return ""
}

func desiredStateToStatuses(desired string) []string {
	switch desired {
	case "running":
		return []string{"Running"}
	case "paused":
		return []string{"Suspended", "Stopped"}
	default:
		return []string{"Running"}
	}
}

func statusToDesiredState(status string) string {
	switch status {
	case "Suspended", "Stopped":
		return "paused"
	default:
		return "running"
	}
}

func (r *ClusterResource) buildAutoPause(ctx context.Context, autoPauseList types.List, diags *diag.Diagnostics) *client.AutoPauseConfig {
	if autoPauseList.IsNull() || autoPauseList.IsUnknown() {
		return nil
	}
	var apModels []AutoPauseModel
	diags.Append(autoPauseList.ElementsAs(ctx, &apModels, false)...)
	if len(apModels) == 0 {
		return nil
	}
	ap := apModels[0]
	cfg := &client.AutoPauseConfig{Enabled: ap.Enabled.ValueBool()}
	setOptionalIntFromInt64(&cfg.IdleTimeoutMinutes, ap.IdleTimeoutMinutes)
	return cfg
}

func connectionInfoAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"public_endpoint":  types.StringType,
		"private_endpoint": types.StringType,
		"listener_port":    types.Int64Type,
	}
}

func (r *ClusterResource) readClusterIntoState(ctx context.Context, warehouseID, clusterID string, state *ClusterResourceModel, diags *diag.Diagnostics) {
	cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			state.ID = types.StringNull()
			return
		}
		diags.AddError(userError("reading cluster", err))
		return
	}

	state.ID = types.StringValue(cl.ClusterID)
	state.WarehouseID = types.StringValue(cl.WarehouseID)
	state.Name = types.StringValue(cl.Name)
	state.Status = types.StringValue(cl.Status)
	state.ClusterType = stringOrNull(cl.ClusterType)
	state.CloudProvider = stringOrNull(cl.CloudProvider)
	state.Region = stringOrNull(cl.Region)
	state.Zone = stringOrNull(cl.Zone)
	state.BillingMethod = stringOrNull(cl.BillingModel)

	// Computed fields from BillingSummary or ClusterItem
	if cl.BillingSummary != nil {
		state.TotalCpu = types.Int64Value(int64(cl.BillingSummary.TotalCpu))
		state.TotalDiskGb = types.Int64Value(int64(cl.BillingSummary.TotalDiskSizeGb))
		state.NodeCount = types.Int64Value(int64(cl.BillingSummary.NodeCount))
	} else {
		state.NodeCount = types.Int64Value(int64(cl.NodeCount))
		state.TotalCpu = types.Int64Null()
		state.TotalDiskGb = types.Int64Null()
	}

	// Flat compute_vcpu and cache_gb from billing pools or cluster-level fields
	if cl.BillingPools != nil && cl.BillingPools.OnDemand != nil {
		state.ComputeVcpu = types.Int64Value(int64(cl.BillingPools.OnDemand.Cpu))
		state.CacheGb = types.Int64Value(int64(cl.BillingPools.OnDemand.DiskSizeGb))
	} else if cl.BillingPools != nil && cl.BillingPools.Subscription != nil {
		state.ComputeVcpu = types.Int64Value(int64(cl.BillingPools.Subscription.Cpu))
		state.CacheGb = types.Int64Value(int64(cl.BillingPools.Subscription.DiskSizeGb))
	} else if cl.BillingSummary != nil {
		state.ComputeVcpu = types.Int64Value(int64(cl.BillingSummary.TotalCpu))
		state.CacheGb = types.Int64Value(int64(cl.BillingSummary.TotalDiskSizeGb))
	}

	if cl.CreatedAt != nil {
		state.CreatedAt = types.StringValue(cl.CreatedAt.Format(time.RFC3339))
	}
	if cl.StartedAt != nil {
		state.StartedAt = types.StringValue(cl.StartedAt.Format(time.RFC3339))
	} else {
		state.StartedAt = types.StringNull()
	}
	if cl.ExpireTime != nil {
		state.ExpireTime = types.StringValue(cl.ExpireTime.Format(time.RFC3339))
	} else {
		state.ExpireTime = types.StringNull()
	}

	state.DesiredState = types.StringValue(statusToDesiredState(cl.Status))

	// Connection info
	if cl.ConnectionInfo != nil {
		obj, d := types.ObjectValue(connectionInfoAttrTypes(), map[string]attr.Value{
			"public_endpoint":  stringOrNull(cl.ConnectionInfo.PublicEndpoint),
			"private_endpoint": stringOrNull(cl.ConnectionInfo.PrivateEndpoint),
			"listener_port":    types.Int64Value(int64(cl.ConnectionInfo.ListenerPort)),
		})
		diags.Append(d...)
		list, d := types.ListValue(types.ObjectType{AttrTypes: connectionInfoAttrTypes()}, []attr.Value{obj})
		diags.Append(d...)
		state.ConnectionInfo = list
	} else {
		state.ConnectionInfo = types.ListNull(types.ObjectType{AttrTypes: connectionInfoAttrTypes()})
	}
}

// --- Simple helpers ---

func stringPtr(s string) *string { return &s }
