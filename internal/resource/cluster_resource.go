package resource

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

var (
	_ resource.Resource                = &ClusterResource{}
	_ resource.ResourceWithImportState = &ClusterResource{}
)

type ClusterResource struct {
	client *client.FormationClient
}

func NewClusterResource() resource.Resource {
	return &ClusterResource{}
}

// --- Terraform model ---

type ClusterResourceModel struct {
	ID               types.String   `tfsdk:"id"`
	WarehouseID      types.String   `tfsdk:"warehouse_id"`
	Name             types.String   `tfsdk:"name"`
	ClusterType      types.String   `tfsdk:"cluster_type"`
	ComputeVcpu      types.Int64    `tfsdk:"compute_vcpu"`
	CacheGb          types.Int64    `tfsdk:"cache_gb"`
	Zone             types.String   `tfsdk:"zone"`
	DesiredState     types.String   `tfsdk:"desired_state"`
	BillingModel    types.String   `tfsdk:"billing_model"`
	Period           types.Int64    `tfsdk:"period"`
	PeriodUnit       types.String   `tfsdk:"period_unit"`
	AutoRenewEnabled types.Int64    `tfsdk:"auto_renew_enabled"`
	AutoPause        types.List     `tfsdk:"auto_pause"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
	// Computed
	Status        types.String `tfsdk:"status"`
	CloudProvider types.String `tfsdk:"cloud_provider"`
	Region        types.String `tfsdk:"region"`
	DiskSumSize   types.Int64  `tfsdk:"disk_sum_size"`
	PayType       types.String `tfsdk:"pay_type"`
	CreatedAt     types.String `tfsdk:"created_at"`
	StartedAt     types.String `tfsdk:"started_at"`
	ExpireTime    types.String `tfsdk:"expire_time"`
	ConnectionInfo types.List  `tfsdk:"connection_info"`
}

type ConnectionInfoModel struct {
	PublicEndpoint  types.String `tfsdk:"public_endpoint"`
	PrivateEndpoint types.String `tfsdk:"private_endpoint"`
	ListenerPort    types.Int64  `tfsdk:"listener_port"`
}

// --- Schema ---

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
				Description: "Cluster type: SQL, COMPUTE, or OBSERVER.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"compute_vcpu": schema.Int64Attribute{
				Description: "Compute capacity in vCPUs.",
				Required:    true,
			},
			"cache_gb": schema.Int64Attribute{
				Description: "Cache capacity in GB.",
				Required:    true,
			},
			"zone": schema.StringAttribute{
				Description: "Availability zone.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"desired_state": schema.StringAttribute{
				Description: "Desired cluster state: running, paused. Changes trigger actions API.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"billing_model": schema.StringAttribute{
				Description: "Billing method (e.g., on_demand, monthly).",
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
			"auto_renew_enabled": schema.Int64Attribute{
				Description: "Auto-renew flag for prepaid billing.",
				Optional:    true,
			},
			// Computed
			"status": schema.StringAttribute{
				Description: "Current observed cluster status.",
				Computed:    true,
			},
			"cloud_provider": schema.StringAttribute{
				Description: "Cloud provider.",
				Computed:    true,
			},
			"region": schema.StringAttribute{
				Description: "Cloud region.",
				Computed:    true,
			},
			"disk_sum_size": schema.Int64Attribute{
				Description: "Current disk capacity in GB.",
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
			"started_at": schema.StringAttribute{
				Description: "Start time in ISO 8601 format.",
				Computed:    true,
			},
			"expire_time": schema.StringAttribute{
				Description: "Expiration time when available.",
				Computed:    true,
			},
			"connection_info": schema.ListNestedAttribute{
				Description: "Cluster connection endpoints (computed).",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"public_endpoint": schema.StringAttribute{
							Description: "Public endpoint address.",
							Computed:    true,
						},
						"private_endpoint": schema.StringAttribute{
							Description: "Private endpoint address.",
							Computed:    true,
						},
						"listener_port": schema.Int64Attribute{
							Description: "TCP listener port.",
							Computed:    true,
						},
					},
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
							Description: "Idle timeout in minutes before auto-pause.",
							Optional:    true,
						},
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
	setOptionalString(&createReq.Zone, plan.Zone)
	setOptionalString(&createReq.BillingModel, plan.BillingModel)
	setOptionalIntFromInt64(&createReq.Period, plan.Period)
	setOptionalString(&createReq.PeriodUnit, plan.PeriodUnit)

	if !plan.AutoRenewEnabled.IsNull() && !plan.AutoRenewEnabled.IsUnknown() {
		v := int(plan.AutoRenewEnabled.ValueInt64())
		createReq.AutoRenewEnabled = &v
	}

	// Auto-pause
	apCfg := r.buildAutoPause(ctx, plan.AutoPause, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	createReq.AutoPause = apCfg

	result, err := r.client.CreateCluster(ctx, warehouseID, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating cluster", err.Error())
		return
	}

	plan.ID = types.StringValue(result.ClusterID)

	// Wait for Running
	_, err = client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
		cl, err := r.client.GetCluster(ctx, warehouseID, result.ClusterID)
		if err != nil {
			return "", err
		}
		return cl.Status, nil
	}, []string{"Running"}, client.FailedStatuses, createTimeout, 15*time.Second)
	if err != nil {
		resp.Diagnostics.AddWarning("Cluster created but not yet Running", err.Error())
	}

	// If desired_state is "paused", pause the cluster after creation
	if !plan.DesiredState.IsNull() && plan.DesiredState.ValueString() == "paused" {
		if err := r.client.OperateCluster(ctx, warehouseID, result.ClusterID, "pause"); err != nil {
			resp.Diagnostics.AddError("Error pausing cluster after creation", err.Error())
			return
		}
		_, _ = client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
			cl, err := r.client.GetCluster(ctx, warehouseID, result.ClusterID)
			if err != nil {
				return "", err
			}
			return cl.Status, nil
		}, []string{"Suspended", "Stopped"}, client.FailedStatuses, createTimeout, 10*time.Second)
	}

	r.readClusterIntoState(ctx, warehouseID, result.ClusterID, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve config-only fields that the API doesn't return
	priorComputeVcpu := state.ComputeVcpu
	priorCacheGb := state.CacheGb
	priorBillingModel := state.BillingModel
	priorAutoPause := state.AutoPause
	priorAutoRenewEnabled := state.AutoRenewEnabled
	priorPeriod := state.Period
	priorPeriodUnit := state.PeriodUnit
	priorTimeouts := state.Timeouts

	r.readClusterIntoState(ctx, state.WarehouseID.ValueString(), state.ID.ValueString(), &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	state.ComputeVcpu = priorComputeVcpu
	state.CacheGb = priorCacheGb
	state.BillingModel = priorBillingModel
	state.AutoPause = priorAutoPause
	state.AutoRenewEnabled = priorAutoRenewEnabled
	state.Period = priorPeriod
	state.PeriodUnit = priorPeriodUnit
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

	updateTimeout, diags := plan.Timeouts.Update(ctx, 20*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	warehouseID := state.WarehouseID.ValueString()
	clusterID := state.ID.ValueString()

	// Handle desired_state changes (pause/resume)
	if !plan.DesiredState.Equal(state.DesiredState) {
		action := r.resolveAction(state.DesiredState.ValueString(), plan.DesiredState.ValueString())
		if action != "" {
			if err := r.client.OperateCluster(ctx, warehouseID, clusterID, action); err != nil {
				resp.Diagnostics.AddError("Error performing cluster action", err.Error())
				return
			}
			targetStatuses := desiredStateToStatuses(plan.DesiredState.ValueString())
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

	// Handle compute_vcpu and cache_gb separately — API does not allow both in one call
	// Only resize when the prior state had a real value (skip on first apply after import
	// where state values are null/zero because the API doesn't return them)
	vcpuChanged := !plan.ComputeVcpu.Equal(state.ComputeVcpu) &&
		!state.ComputeVcpu.IsNull() && state.ComputeVcpu.ValueInt64() > 0
	cacheChanged := !plan.CacheGb.Equal(state.CacheGb) &&
		!state.CacheGb.IsNull() && state.CacheGb.ValueInt64() > 0

	if vcpuChanged {
		v := int(plan.ComputeVcpu.ValueInt64())
		if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, &client.UpdateClusterRequest{ComputeVcpu: &v}); err != nil {
			resp.Diagnostics.AddError("Error resizing cluster (compute_vcpu)", err.Error())
			return
		}
		_, err := client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
			cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
			if err != nil {
				return "", err
			}
			return cl.Status, nil
		}, client.StableStatuses, client.FailedStatuses, updateTimeout, 15*time.Second)
		if err != nil {
			resp.Diagnostics.AddWarning("Cluster vcpu resize may still be in progress", err.Error())
		}
	}

	if cacheChanged {
		v := int(plan.CacheGb.ValueInt64())
		if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, &client.UpdateClusterRequest{CacheGb: &v}); err != nil {
			resp.Diagnostics.AddError("Error resizing cluster (cache_gb)", err.Error())
			return
		}
		_, err := client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
			cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
			if err != nil {
				return "", err
			}
			return cl.Status, nil
		}, client.StableStatuses, client.FailedStatuses, updateTimeout, 15*time.Second)
		if err != nil {
			resp.Diagnostics.AddWarning("Cluster cache resize may still be in progress", err.Error())
		}
	}

	// Handle other attribute updates (name, billing, auto_pause)
	needsUpdate := !plan.Name.Equal(state.Name) ||
		!plan.BillingModel.Equal(state.BillingModel) ||
		!plan.Period.Equal(state.Period) ||
		!plan.PeriodUnit.Equal(state.PeriodUnit) ||
		!plan.AutoRenewEnabled.Equal(state.AutoRenewEnabled) ||
		!plan.AutoPause.Equal(state.AutoPause)

	if needsUpdate {
		updateReq := &client.UpdateClusterRequest{}
		if !plan.Name.Equal(state.Name) {
			s := plan.Name.ValueString()
			updateReq.Name = &s
		}
		setOptionalString(&updateReq.BillingModel, plan.BillingModel)
		setOptionalIntFromInt64(&updateReq.Period, plan.Period)
		setOptionalString(&updateReq.PeriodUnit, plan.PeriodUnit)
		if !plan.AutoRenewEnabled.IsNull() && !plan.AutoRenewEnabled.IsUnknown() {
			v := int(plan.AutoRenewEnabled.ValueInt64())
			updateReq.AutoRenewEnabled = &v
		}
		updateReq.AutoPause = r.buildAutoPause(ctx, plan.AutoPause, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}

		if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, updateReq); err != nil {
			resp.Diagnostics.AddError("Error updating cluster", err.Error())
			return
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
		resp.Diagnostics.AddError("Error deleting cluster", err.Error())
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

func (r *ClusterResource) readClusterIntoState(ctx context.Context, warehouseID, clusterID string, state *ClusterResourceModel, diags *diag.Diagnostics) {
	cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			state.ID = types.StringNull()
			return
		}
		diags.AddError("Error reading cluster", err.Error())
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
	state.PayType = stringOrNull(cl.PayType)

	if cl.DiskSumSize > 0 {
		state.DiskSumSize = types.Int64Value(int64(cl.DiskSumSize))
	} else {
		state.DiskSumSize = types.Int64Null()
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

	// Map status to desired_state
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
	cfg := &client.AutoPauseConfig{
		Enabled: ap.Enabled.ValueBool(),
	}
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

// resolveAction determines the API action needed to transition between states.
func (r *ClusterResource) resolveAction(currentDesired, newDesired string) string {
	switch newDesired {
	case "running":
		return "resume"
	case "paused":
		return "pause"
	}
	return ""
}

// desiredStateToStatuses maps desired_state to the expected API statuses.
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

// statusToDesiredState maps API status to the Terraform desired_state value.
func statusToDesiredState(status string) string {
	switch status {
	case "Suspended", "Stopped":
		return "paused"
	default:
		return "running"
	}
}
