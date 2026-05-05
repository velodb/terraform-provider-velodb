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
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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

// --- Terraform models ---

type ClusterResourceModel struct {
	ID            types.String   `tfsdk:"id"`
	WarehouseID   types.String   `tfsdk:"warehouse_id"`
	Name          types.String   `tfsdk:"name"`
	ClusterType   types.String   `tfsdk:"cluster_type"`
	Zone          types.String   `tfsdk:"zone"`
	DesiredState  types.String   `tfsdk:"desired_state"`
	RebootTrigger types.Int64    `tfsdk:"reboot_trigger"`
	Subscription  types.List     `tfsdk:"subscription"`
	OnDemand      types.List     `tfsdk:"on_demand"`
	AutoPause     types.List     `tfsdk:"auto_pause"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
	// Computed
	Status                types.String `tfsdk:"status"`
	CloudProvider         types.String `tfsdk:"cloud_provider"`
	Region                types.String `tfsdk:"region"`
	IsMixedBilling        types.Bool   `tfsdk:"is_mixed_billing"`
	TotalCpu              types.Int64  `tfsdk:"total_cpu"`
	TotalDiskGb           types.Int64  `tfsdk:"total_disk_gb"`
	NodeCount             types.Int64  `tfsdk:"node_count"`
	OnDemandNodeCount     types.Int64  `tfsdk:"on_demand_node_count"`
	SubscriptionNodeCount types.Int64  `tfsdk:"subscription_node_count"`
	CreatedAt             types.String `tfsdk:"created_at"`
	StartedAt             types.String `tfsdk:"started_at"`
	ExpireTime            types.String `tfsdk:"expire_time"`
	ConnectionInfo        types.List   `tfsdk:"connection_info"`
}

type SubscriptionPoolModel struct {
	ComputeVcpu types.Int64  `tfsdk:"compute_vcpu"`
	CacheGb     types.Int64  `tfsdk:"cache_gb"`
	Period      types.Int64  `tfsdk:"period"`
	PeriodUnit  types.String `tfsdk:"period_unit"`
	AutoRenew   types.Bool   `tfsdk:"auto_renew"`
}

type OnDemandPoolModel struct {
	ComputeVcpu types.Int64 `tfsdk:"compute_vcpu"`
	CacheGb     types.Int64 `tfsdk:"cache_gb"`
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
		Description: "Manages a VeloDB Cloud cluster within a warehouse. Supports pure on-demand, pure subscription, or mixed billing via the subscription{} and on_demand{} blocks.",
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
			"zone": schema.StringAttribute{
				Description: "Availability zone.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"desired_state": schema.StringAttribute{
				Description: "Desired cluster state: running or paused.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cloud_provider": schema.StringAttribute{
				Computed: true, Description: "Cloud provider.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"region": schema.StringAttribute{
				Computed: true, Description: "Cloud region.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			// Pool-derived computed fields: these change whenever vcpu/cache_gb/pool composition
			// changes, so we cannot use UseStateForUnknown — the framework would lie about the
			// expected value and reject the apply with "inconsistent result". They show
			// "(known after apply)" on every update plan, which is a cosmetic diff but correct.
			"is_mixed_billing": schema.BoolAttribute{
				Description: "True when the cluster has both on_demand and subscription pools.",
				Computed:    true,
			},
			"total_cpu": schema.Int64Attribute{
				Computed: true, Description: "Total CPU across all pools.",
			},
			"total_disk_gb": schema.Int64Attribute{
				Computed: true, Description: "Total disk GB across all pools.",
			},
			"node_count": schema.Int64Attribute{
				Computed: true, Description: "Total node count.",
			},
			"on_demand_node_count": schema.Int64Attribute{
				Description: "Node count in the on-demand pool.",
				Computed:    true,
			},
			"subscription_node_count": schema.Int64Attribute{
				Description: "Node count in the subscription pool.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Computed: true, Description: "Creation time in RFC 3339 format.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"started_at": schema.StringAttribute{
				Computed: true, Description: "Start time in RFC 3339 format.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"expire_time": schema.StringAttribute{
				Computed: true, Description: "Expiration time when applicable.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
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
			"subscription": schema.ListNestedBlock{
				Description: "Subscription billing pool. Required when the cluster uses reserved/subscription capacity.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"compute_vcpu": schema.Int64Attribute{
							Description: "vCPU capacity of the subscription pool. Minimum 4. Valid values are 4, 8, 16, and multiples of 16 above that.",
							Required:    true,
							Validators:  []validator.Int64{int64validator.AtLeast(4)},
						},
						"cache_gb": schema.Int64Attribute{
							Description: "Cache GB of the subscription pool. Optional — when omitted, the API auto-scales disk proportionally to compute_vcpu.",
							Optional:    true,
							Computed:    true,
							Validators:  []validator.Int64{int64validator.AtLeast(100)},
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						"period": schema.Int64Attribute{
							Description: "Subscription period length.",
							Required:    true,
						},
						"period_unit": schema.StringAttribute{
							Description: "Period unit: Month or Year.",
							Required:    true,
						},
						"auto_renew": schema.BoolAttribute{
							Description: "Whether the subscription auto-renews at expiration.",
							Optional:    true,
							Computed:    true,
						},
					},
				},
			},
			"on_demand": schema.ListNestedBlock{
				Description: "On-demand billing pool. Required when the cluster uses pay-as-you-go capacity.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"compute_vcpu": schema.Int64Attribute{
							Description: "vCPU capacity of the on-demand pool. Minimum 4. Valid values are 4, 8, 16, and multiples of 16 above that.",
							Required:    true,
							Validators:  []validator.Int64{int64validator.AtLeast(4)},
						},
						"cache_gb": schema.Int64Attribute{
							Description: "Cache GB of the on-demand pool. Optional — when omitted, the API auto-scales disk proportionally to compute_vcpu.",
							Optional:    true,
							Computed:    true,
							Validators:  []validator.Int64{int64validator.AtLeast(100)},
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
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

	sub := r.extractSubscriptionPool(ctx, plan.Subscription, &resp.Diagnostics)
	od := r.extractOnDemandPool(ctx, plan.OnDemand, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if sub == nil && od == nil {
		resp.Diagnostics.AddError("Invalid cluster configuration", "At least one of subscription{} or on_demand{} blocks must be provided.")
		return
	}

	// Primary pool to create with. Prefer on_demand if present; else subscription.
	// Rationale: on_demand creation is simpler; if mixed, we'll add subscription via convert.
	warehouseID := plan.WarehouseID.ValueString()
	var clusterID string

	if od != nil {
		// Create pure on_demand first
		createReq := &client.CreateClusterRequest{
			Name:         plan.Name.ValueString(),
			ClusterType:  plan.ClusterType.ValueString(),
			ComputeVcpu:  int(od.ComputeVcpu.ValueInt64()),
			CacheGb:      int(od.CacheGb.ValueInt64()),
			BillingModel: stringPtr("on_demand"),
		}
		setOptionalString(&createReq.Zone, plan.Zone)
		createReq.AutoPause = r.buildAutoPause(ctx, plan.AutoPause, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}

		result, err := r.client.CreateCluster(ctx, warehouseID, createReq)
		if err != nil {
			resp.Diagnostics.AddError("Error creating cluster (on_demand)", err.Error())
			return
		}
		clusterID = result.ClusterID
	} else {
		// Create pure subscription
		createReq := &client.CreateClusterRequest{
			Name:         plan.Name.ValueString(),
			ClusterType:  plan.ClusterType.ValueString(),
			ComputeVcpu:  int(sub.ComputeVcpu.ValueInt64()),
			CacheGb:      int(sub.CacheGb.ValueInt64()),
			BillingModel: stringPtr("subscription"),
			Period:       intPtr(int(sub.Period.ValueInt64())),
			PeriodUnit:   stringPtr(sub.PeriodUnit.ValueString()),
		}
		if !sub.AutoRenew.IsNull() && !sub.AutoRenew.IsUnknown() {
			b := sub.AutoRenew.ValueBool()
			createReq.AutoRenew = &b
		}
		setOptionalString(&createReq.Zone, plan.Zone)
		createReq.AutoPause = r.buildAutoPause(ctx, plan.AutoPause, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}

		result, err := r.client.CreateCluster(ctx, warehouseID, createReq)
		if err != nil {
			resp.Diagnostics.AddError("Error creating cluster (subscription)", err.Error())
			return
		}
		clusterID = result.ClusterID
	}

	plan.ID = types.StringValue(clusterID)

	// Wait to Running
	_, err := client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
		cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
		if err != nil {
			return "", err
		}
		return cl.Status, nil
	}, []string{"Running"}, client.FailedStatuses, createTimeout, 15*time.Second)
	if err != nil {
		resp.Diagnostics.AddWarning("Cluster created but not yet Running", err.Error())
	}

	// Mixed billing: if both pools configured, add the subscription pool via PATCH.
	// API rule: computeVcpu and cacheGb cannot be updated at the same time, so the
	// "add subscription pool" call splits into two sequential PATCHes — first vcpu
	// + period (which establishes the subscription pool), then cacheGb.
	if sub != nil && od != nil {
		vcpu := int(sub.ComputeVcpu.ValueInt64())
		cache := int(sub.CacheGb.ValueInt64())
		period := int(sub.Period.ValueInt64())
		periodUnit := sub.PeriodUnit.ValueString()
		// 1. Establish subscription pool with vcpu (no cacheGb in this call).
		addReq := &client.UpdateClusterRequest{
			BillingModel: stringPtr("subscription"),
			ComputeVcpu:  &vcpu,
			Period:       &period,
			PeriodUnit:   &periodUnit,
		}
		if !sub.AutoRenew.IsNull() && !sub.AutoRenew.IsUnknown() {
			b := sub.AutoRenew.ValueBool()
			addReq.AutoRenew = &b
		}
		if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, addReq); err != nil {
			resp.Diagnostics.AddError("Error adding subscription pool after cluster creation", err.Error())
			return
		}
		_, _ = client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
			cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
			if err != nil {
				return "", err
			}
			return cl.Status, nil
		}, client.StableStatuses, client.FailedStatuses, createTimeout, 15*time.Second)
		// 2. Resize subscription cacheGb in a separate PATCH (skip if already at target).
		if cache > 0 {
			diskReq := &client.UpdateClusterRequest{
				BillingModel: stringPtr("subscription"),
				CacheGb:      &cache,
			}
			if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, diskReq); err != nil {
				resp.Diagnostics.AddError("Error sizing subscription pool cacheGb after cluster creation", err.Error())
				return
			}
			_, _ = client.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
				cl, err := r.client.GetCluster(ctx, warehouseID, clusterID)
				if err != nil {
					return "", err
				}
				return cl.Status, nil
			}, client.StableStatuses, client.FailedStatuses, createTimeout, 15*time.Second)
		}
	}

	// Handle initial desired_state = paused
	if !plan.DesiredState.IsNull() && plan.DesiredState.ValueString() == "paused" {
		if err := r.client.PauseCluster(ctx, warehouseID, clusterID); err != nil {
			resp.Diagnostics.AddError("Error pausing cluster after creation", err.Error())
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
	// Preserve plan values for subscription.auto_renew and subscription.period
	if sub != nil && !plan.Subscription.IsNull() {
		var pools []SubscriptionPoolModel
		resp.Diagnostics.Append(plan.Subscription.ElementsAs(ctx, &pools, false)...)
		if len(pools) > 0 {
			pools[0].AutoRenew = sub.AutoRenew
			if pools[0].Period.IsNull() || pools[0].Period.ValueInt64() == 0 {
				pools[0].Period = sub.Period
			}
			newList, d := types.ListValueFrom(ctx, plan.Subscription.ElementType(ctx), pools)
			resp.Diagnostics.Append(d...)
			plan.Subscription = newList
		}
	}
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
	priorSubAutoRenew := types.BoolNull()
	priorSubPeriod := types.Int64Null()
	if priorSub := r.extractSubscriptionPool(ctx, state.Subscription, &resp.Diagnostics); priorSub != nil {
		priorSubAutoRenew = priorSub.AutoRenew
		priorSubPeriod = priorSub.Period
	}

	r.readClusterIntoState(ctx, state.WarehouseID.ValueString(), state.ID.ValueString(), &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// readClusterIntoState derives Subscription and OnDemand blocks from billingPools.
	// Preserve auto_renew (not returned by API) and period (not always returned by API).
	if !state.Subscription.IsNull() && !state.Subscription.IsUnknown() {
		var newPools []SubscriptionPoolModel
		resp.Diagnostics.Append(state.Subscription.ElementsAs(ctx, &newPools, false)...)
		if len(newPools) > 0 {
			if !priorSubAutoRenew.IsNull() {
				newPools[0].AutoRenew = priorSubAutoRenew
			}
			// If API didn't return period, use prior value
			if newPools[0].Period.IsNull() || newPools[0].Period.ValueInt64() == 0 {
				if !priorSubPeriod.IsNull() {
					newPools[0].Period = priorSubPeriod
				}
			}
			newList, d := types.ListValueFrom(ctx, state.Subscription.ElementType(ctx), newPools)
			resp.Diagnostics.Append(d...)
			state.Subscription = newList
		}
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

	planSub := r.extractSubscriptionPool(ctx, plan.Subscription, &resp.Diagnostics)
	planOd := r.extractOnDemandPool(ctx, plan.OnDemand, &resp.Diagnostics)
	stateSub := r.extractSubscriptionPool(ctx, state.Subscription, &resp.Diagnostics)
	stateOd := r.extractOnDemandPool(ctx, state.OnDemand, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Detect pool additions/removals
	subAdded := stateSub == nil && planSub != nil
	odAdded := stateOd == nil && planOd != nil
	subRemoved := stateSub != nil && planSub == nil
	odRemoved := stateOd != nil && planOd == nil

	// Pool removal: API accepts PATCH with billingModel=<pool>, computeVcpu=0 to remove that pool.
	// At least one pool must remain.
	if subRemoved && odRemoved {
		resp.Diagnostics.AddError(
			"Cannot remove all billing pools",
			"At least one of subscription{} or on_demand{} must be present. To destroy the cluster, remove the resource entirely.",
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
				resp.Diagnostics.AddError("Error reading cluster before action", err.Error())
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
					resp.Diagnostics.AddError("Error performing cluster action", err.Error())
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
			resp.Diagnostics.AddError("Error rebooting cluster", err.Error())
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

	// 2b. Pool removal: PATCH billingModel=<pool>, computeVcpu=0 to remove a pool
	if subRemoved {
		zero := 0
		req := &client.UpdateClusterRequest{
			BillingModel: stringPtr("subscription"),
			ComputeVcpu:  &zero,
		}
		if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, req); err != nil {
			resp.Diagnostics.AddError("Error removing subscription pool", err.Error())
			return
		}
		r.waitStable(ctx, warehouseID, clusterID, updateTimeout, &resp.Diagnostics)
	}
	if odRemoved {
		zero := 0
		req := &client.UpdateClusterRequest{
			BillingModel: stringPtr("on_demand"),
			ComputeVcpu:  &zero,
		}
		if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, req); err != nil {
			resp.Diagnostics.AddError("Error removing on_demand pool", err.Error())
			return
		}
		r.waitStable(ctx, warehouseID, clusterID, updateTimeout, &resp.Diagnostics)
	}

	// 3. Add subscription pool to pure on_demand cluster.
	// API rule: computeVcpu and cacheGb cannot be updated at the same time, so split
	// into two sequential PATCHes — first establish the subscription pool with vcpu,
	// then size cacheGb separately.
	if subAdded && stateOd != nil {
		vcpu := int(planSub.ComputeVcpu.ValueInt64())
		cache := int(planSub.CacheGb.ValueInt64())
		period := int(planSub.Period.ValueInt64())
		periodUnit := planSub.PeriodUnit.ValueString()
		addReq := &client.UpdateClusterRequest{
			BillingModel: stringPtr("subscription"),
			ComputeVcpu:  &vcpu,
			Period:       &period,
			PeriodUnit:   &periodUnit,
		}
		if !planSub.AutoRenew.IsNull() && !planSub.AutoRenew.IsUnknown() {
			b := planSub.AutoRenew.ValueBool()
			addReq.AutoRenew = &b
		}
		if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, addReq); err != nil {
			resp.Diagnostics.AddError("Error adding subscription pool", err.Error())
			return
		}
		r.waitStable(ctx, warehouseID, clusterID, updateTimeout, &resp.Diagnostics)
		if cache > 0 {
			diskReq := &client.UpdateClusterRequest{
				BillingModel: stringPtr("subscription"),
				CacheGb:      &cache,
			}
			if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, diskReq); err != nil {
				resp.Diagnostics.AddError("Error sizing subscription pool cacheGb", err.Error())
				return
			}
			r.waitStable(ctx, warehouseID, clusterID, updateTimeout, &resp.Diagnostics)
		}
	}

	// 4. Add on_demand pool to pure subscription cluster
	if odAdded && stateSub != nil {
		v := int(planOd.ComputeVcpu.ValueInt64())
		req := &client.UpdateClusterRequest{
			BillingModel: stringPtr("on_demand"),
			ComputeVcpu:  &v,
		}
		if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, req); err != nil {
			resp.Diagnostics.AddError("Error adding on_demand pool", err.Error())
			return
		}
		r.waitStable(ctx, warehouseID, clusterID, updateTimeout, &resp.Diagnostics)
	}

	// 5. Resize on_demand pool (when both old and new exist).
	// API rule: vcpu and cache_gb cannot move in the same PATCH. The API also auto-scales
	// disk proportionally to a vcpu change, which would clobber a user's pinned cache_gb.
	// Strategy: if vcpu changes, do the vcpu PATCH first; then if the user pinned cache_gb
	// to something other than the auto-scaled result, send a second PATCH to enforce it.
	if stateOd != nil && planOd != nil && !odAdded {
		vcpuChanged := !planOd.ComputeVcpu.Equal(stateOd.ComputeVcpu)
		if vcpuChanged {
			v := int(planOd.ComputeVcpu.ValueInt64())
			req := &client.UpdateClusterRequest{
				BillingModel: stringPtr("on_demand"),
				ComputeVcpu:  &v,
			}
			if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, req); err != nil {
				resp.Diagnostics.AddError("Error resizing on_demand compute_vcpu", err.Error())
				return
			}
			r.waitStable(ctx, warehouseID, clusterID, updateTimeout, &resp.Diagnostics)
		}
		// Enforce the user's cache_gb when (a) it changed in config, or (b) vcpu changed and the
		// user pinned cache_gb (so auto-scale would otherwise clobber it).
		userPinnedCache := !planOd.CacheGb.IsNull() && !planOd.CacheGb.IsUnknown()
		cacheChanged := !planOd.CacheGb.Equal(stateOd.CacheGb)
		if userPinnedCache && (cacheChanged || vcpuChanged) {
			v := int(planOd.CacheGb.ValueInt64())
			req := &client.UpdateClusterRequest{
				BillingModel: stringPtr("on_demand"),
				CacheGb:      &v,
			}
			if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, req); err != nil {
				// "no cluster changes" is normal when auto-scale already arrived at the target value.
				if apiErr, ok := err.(*client.APIError); !ok || !strings.Contains(apiErr.Message, "no cluster changes") {
					resp.Diagnostics.AddError("Error resizing on_demand cache_gb", err.Error())
					return
				}
			} else {
				r.waitStable(ctx, warehouseID, clusterID, updateTimeout, &resp.Diagnostics)
			}
		}
	}

	// 6. Resize subscription pool (vcpu, cache, or period/auto_renew change)
	if stateSub != nil && planSub != nil && !subAdded {
		if !planSub.PeriodUnit.Equal(stateSub.PeriodUnit) {
			resp.Diagnostics.AddError(
				"period_unit change requires replacement",
				"Changing subscription.period_unit is not supported in place. Taint and recreate the cluster.",
			)
			return
		}
		if !planSub.ComputeVcpu.Equal(stateSub.ComputeVcpu) {
			v := int(planSub.ComputeVcpu.ValueInt64())
			req := &client.UpdateClusterRequest{
				BillingModel: stringPtr("subscription"),
				ComputeVcpu:  &v,
			}
			if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, req); err != nil {
				resp.Diagnostics.AddError("Error resizing subscription compute_vcpu", err.Error())
				return
			}
			r.waitStable(ctx, warehouseID, clusterID, updateTimeout, &resp.Diagnostics)
		}
		if !planSub.CacheGb.Equal(stateSub.CacheGb) {
			v := int(planSub.CacheGb.ValueInt64())
			req := &client.UpdateClusterRequest{
				BillingModel: stringPtr("subscription"),
				CacheGb:      &v,
			}
			if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, req); err != nil {
				resp.Diagnostics.AddError("Error resizing subscription cache_gb", err.Error())
				return
			}
			r.waitStable(ctx, warehouseID, clusterID, updateTimeout, &resp.Diagnostics)
		}
		// Period or auto_renew change — skip if state is stale (period=0 indicates API didn't return it)
		stateStalePeriod := stateSub.Period.IsNull() || stateSub.Period.ValueInt64() == 0
		periodChanged := !stateStalePeriod && !planSub.Period.Equal(stateSub.Period)
		autoRenewChanged := !planSub.AutoRenew.IsNull() && !planSub.AutoRenew.IsUnknown() &&
			!stateSub.AutoRenew.IsNull() && !planSub.AutoRenew.Equal(stateSub.AutoRenew)
		if periodChanged || autoRenewChanged {
			p := int(planSub.Period.ValueInt64())
			pu := planSub.PeriodUnit.ValueString()
			req := &client.UpdateClusterRequest{
				BillingModel: stringPtr("subscription"),
				Period:       &p,
				PeriodUnit:   &pu,
			}
			if !planSub.AutoRenew.IsNull() && !planSub.AutoRenew.IsUnknown() {
				b := planSub.AutoRenew.ValueBool()
				req.AutoRenew = &b
			}
			if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, req); err != nil {
				resp.Diagnostics.AddError("Error updating subscription period/auto_renew", err.Error())
				return
			}
		}
	}

	// 7. Name change
	if !plan.Name.Equal(state.Name) {
		s := plan.Name.ValueString()
		if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, &client.UpdateClusterRequest{Name: &s}); err != nil {
			resp.Diagnostics.AddError("Error updating cluster name", err.Error())
			return
		}
	}

	// 8. auto_pause change
	if !plan.AutoPause.Equal(state.AutoPause) {
		ap := r.buildAutoPause(ctx, plan.AutoPause, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		if ap != nil {
			if err := r.client.UpdateCluster(ctx, warehouseID, clusterID, &client.UpdateClusterRequest{AutoPause: ap}); err != nil {
				resp.Diagnostics.AddError("Error updating auto_pause", err.Error())
				return
			}
		}
	}

	r.readClusterIntoState(ctx, warehouseID, clusterID, &plan, &resp.Diagnostics)
	// Preserve plan values for subscription.auto_renew and subscription.period
	// (API doesn't always return them via billingPools)
	if planSub != nil && !plan.Subscription.IsNull() {
		var pools []SubscriptionPoolModel
		resp.Diagnostics.Append(plan.Subscription.ElementsAs(ctx, &pools, false)...)
		if len(pools) > 0 {
			pools[0].AutoRenew = planSub.AutoRenew
			if pools[0].Period.IsNull() || pools[0].Period.ValueInt64() == 0 {
				pools[0].Period = planSub.Period
			}
			newList, d := types.ListValueFrom(ctx, plan.Subscription.ElementType(ctx), pools)
			resp.Diagnostics.Append(d...)
			plan.Subscription = newList
		}
	}
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

func (r *ClusterResource) extractSubscriptionPool(ctx context.Context, list types.List, diags *diag.Diagnostics) *SubscriptionPoolModel {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var pools []SubscriptionPoolModel
	diags.Append(list.ElementsAs(ctx, &pools, false)...)
	if len(pools) == 0 {
		return nil
	}
	return &pools[0]
}

func (r *ClusterResource) extractOnDemandPool(ctx context.Context, list types.List, diags *diag.Diagnostics) *OnDemandPoolModel {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var pools []OnDemandPoolModel
	diags.Append(list.ElementsAs(ctx, &pools, false)...)
	if len(pools) == 0 {
		return nil
	}
	return &pools[0]
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

func subscriptionPoolObjectType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"compute_vcpu": types.Int64Type,
			"cache_gb":     types.Int64Type,
			"period":       types.Int64Type,
			"period_unit":  types.StringType,
			"auto_renew":   types.BoolType,
		},
	}
}

func onDemandPoolObjectType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"compute_vcpu": types.Int64Type,
			"cache_gb":     types.Int64Type,
		},
	}
}

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

	// Mixed-billing computed fields
	if cl.BillingSummary != nil {
		state.IsMixedBilling = types.BoolValue(cl.BillingSummary.IsMixedBilling)
		state.TotalCpu = types.Int64Value(int64(cl.BillingSummary.TotalCpu))
		state.TotalDiskGb = types.Int64Value(int64(cl.BillingSummary.TotalDiskSizeGb))
		state.NodeCount = types.Int64Value(int64(cl.BillingSummary.NodeCount))
		state.OnDemandNodeCount = types.Int64Value(int64(cl.BillingSummary.OnDemandNodeCount))
		state.SubscriptionNodeCount = types.Int64Value(int64(cl.BillingSummary.SubscriptionNodeCount))
	} else {
		// Fall back to ClusterItem fields
		state.IsMixedBilling = types.BoolValue(false)
		state.NodeCount = types.Int64Value(int64(cl.NodeCount))
		state.OnDemandNodeCount = types.Int64Value(int64(cl.OnDemandNodeCount))
		state.SubscriptionNodeCount = types.Int64Value(int64(cl.SubscriptionNodeCount))
		state.TotalCpu = types.Int64Null()
		state.TotalDiskGb = types.Int64Null()
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

	// Billing pools — derive from API billingPools for drift detection
	subObjType := subscriptionPoolObjectType()
	odObjType := onDemandPoolObjectType()

	if cl.BillingPools != nil && cl.BillingPools.Subscription != nil {
		sp := cl.BillingPools.Subscription
		obj, d := types.ObjectValue(subObjType.AttrTypes, map[string]attr.Value{
			"compute_vcpu": types.Int64Value(int64(sp.Cpu)),
			"cache_gb":     types.Int64Value(int64(sp.DiskSizeGb)),
			"period":       types.Int64Value(int64(sp.Period)),
			"period_unit":  stringOrNull(sp.PeriodUnit),
			"auto_renew":   types.BoolNull(), // API doesn't return this; preserved by Read()
		})
		diags.Append(d...)
		lst, d := types.ListValue(subObjType, []attr.Value{obj})
		diags.Append(d...)
		state.Subscription = lst
	} else {
		state.Subscription = types.ListNull(subObjType)
	}

	if cl.BillingPools != nil && cl.BillingPools.OnDemand != nil {
		op := cl.BillingPools.OnDemand
		obj, d := types.ObjectValue(odObjType.AttrTypes, map[string]attr.Value{
			"compute_vcpu": types.Int64Value(int64(op.Cpu)),
			"cache_gb":     types.Int64Value(int64(op.DiskSizeGb)),
		})
		diags.Append(d...)
		lst, d := types.ListValue(odObjType, []attr.Value{obj})
		diags.Append(d...)
		state.OnDemand = lst
	} else {
		state.OnDemand = types.ListNull(odObjType)
	}

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
func intPtr(i int) *int          { return &i }
