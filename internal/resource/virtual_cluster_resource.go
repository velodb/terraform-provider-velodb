package resource

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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

type VirtualClusterResource struct {
	client *client.FormationClient
}

func NewVirtualClusterResource() resource.Resource {
	return &VirtualClusterResource{}
}

var (
	_ resource.Resource                   = &VirtualClusterResource{}
	_ resource.ResourceWithConfigure      = &VirtualClusterResource{}
	_ resource.ResourceWithImportState    = &VirtualClusterResource{}
	_ resource.ResourceWithModifyPlan     = &VirtualClusterResource{}
	_ resource.ResourceWithValidateConfig = &VirtualClusterResource{}
)

type VirtualClusterResourceModel struct {
	ID                      types.String   `tfsdk:"id"`
	WarehouseID             types.String   `tfsdk:"warehouse_id"`
	Name                    types.String   `tfsdk:"name"`
	DesiredActiveClusterID  types.String   `tfsdk:"desired_active_cluster_id"`
	DesiredStandbyClusterID types.String   `tfsdk:"desired_standby_cluster_id"`
	PairReconcileTrigger    types.Int64    `tfsdk:"pair_reconcile_trigger"`
	ActiveClusterID         types.String   `tfsdk:"active_cluster_id"`
	StandbyClusterID        types.String   `tfsdk:"standby_cluster_id"`
	Status                  types.String   `tfsdk:"status"`
	AtRisk                  types.Bool     `tfsdk:"at_risk"`
	RiskReasons             types.Set      `tfsdk:"risk_reasons"`
	CreatedAt               types.String   `tfsdk:"created_at"`
	UpdatedAt               types.String   `tfsdk:"updated_at"`
	Timeouts                timeouts.Value `tfsdk:"timeouts"`
}

func (r *VirtualClusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_cluster"
}

func (r *VirtualClusterResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an active and standby virtual cluster in a VeloDB warehouse.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Virtual-cluster identifier.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"warehouse_id": schema.StringAttribute{Required: true, Description: "Parent warehouse identifier. Cannot be changed in place.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
			"name": schema.StringAttribute{
				Required: true, Description: "Virtual-cluster name.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 32),
					stringvalidator.RegexMatches(regexp.MustCompile(`^[A-Za-z](?:[A-Za-z0-9]|_[A-Za-z0-9])*_?$`), "must start with a letter and contain only letters, numbers, and nonconsecutive underscores"),
				},
			},
			"desired_active_cluster_id":  schema.StringAttribute{Required: true, Description: "Requested active physical-cluster ID; changes explicitly update the full desired pair.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
			"desired_standby_cluster_id": schema.StringAttribute{Required: true, Description: "Requested standby physical-cluster ID; changes explicitly update the full desired pair.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
			"pair_reconcile_trigger":     schema.Int64Attribute{Optional: true, Description: "Change this value to explicitly reapply the desired pair after failover."},
			"active_cluster_id":          schema.StringAttribute{Computed: true, Description: "Observed active physical-cluster ID."},
			"standby_cluster_id":         schema.StringAttribute{Computed: true, Description: "Observed standby physical-cluster ID."},
			"status":                     schema.StringAttribute{Computed: true, Description: "Observed virtual-cluster lifecycle status."},
			"at_risk":                    schema.BoolAttribute{Computed: true, Description: "Whether the virtual cluster is currently reported at risk."},
			"risk_reasons":               schema.SetAttribute{Computed: true, ElementType: types.StringType, Description: "Current risk reason codes; empty outside Running does not prove health."},
			"created_at":                 schema.StringAttribute{Computed: true, Description: "Creation time in RFC 3339 format when available."},
			"updated_at":                 schema.StringAttribute{Computed: true, Description: "Last update time in RFC 3339 format when available."},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *VirtualClusterResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var activeID, standbyID types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("desired_active_cluster_id"), &activeID)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("desired_standby_cluster_id"), &standbyID)...)
	if !activeID.IsNull() && !activeID.IsUnknown() && !standbyID.IsNull() && !standbyID.IsUnknown() && activeID.Equal(standbyID) {
		resp.Diagnostics.AddError("Invalid virtual-cluster members", "The active and standby physical-cluster IDs must be distinct.")
	}
}

func (r *VirtualClusterResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() {
		return
	}
	var planned, prior VirtualClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if req.Plan.Raw.IsNull() {
		resp.Diagnostics.AddWarning("Virtual cluster will be destroyed", fmt.Sprintf(
			"Terraform will delete virtual cluster %q (ID %q), ending routing through its endpoint and automatic failover. "+
				"A terraform destroy -target on a referenced physical member can also delete its dependent virtual cluster. "+
				"Review every resource in the destroy plan. To block accidental deletion, add lifecycle { prevent_destroy = true } to the virtual-cluster resource. "+
				"This warning does not block destruction, including with -auto-approve.", prior.Name.ValueString(), prior.ID.ValueString()))
		return
	}
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planned)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !planned.WarehouseID.Equal(prior.WarehouseID) {
		resp.Diagnostics.AddAttributeError(path.Root("warehouse_id"), "Virtual-cluster warehouse cannot be changed", "Restore the original warehouse ID. Automatic replacement is not supported; create a separate virtual cluster for migration.")
		return
	}
	if virtualClusterPairChanged(planned, prior) {
		warnVirtualClusterActiveChange(&resp.Diagnostics, prior.ActiveClusterID, prior.StandbyClusterID, planned.DesiredActiveClusterID, planned.DesiredStandbyClusterID)
	}
}

func virtualClusterPairChanged(planned, prior VirtualClusterResourceModel) bool {
	return !planned.DesiredActiveClusterID.Equal(prior.DesiredActiveClusterID) ||
		!planned.DesiredStandbyClusterID.Equal(prior.DesiredStandbyClusterID) || !planned.PairReconcileTrigger.Equal(prior.PairReconcileTrigger)
}

func warnVirtualClusterActiveChange(diags *diag.Diagnostics, active, standby, requestedActive, requestedStandby types.String) {
	if active.IsNull() || active.IsUnknown() || active.ValueString() == "" {
		return
	}
	if requestedActive.IsUnknown() {
		diags.AddWarning("Virtual-cluster active change not yet known", fmt.Sprintf("Current active cluster: %q. The requested active cluster ID is not yet known, so this pair update may change the active cluster. The final ID will be resolved during apply. Review whether changing the active cluster is intended.", active.ValueString()))
		return
	}
	if requestedActive.IsNull() || requestedActive.ValueString() == "" || active.Equal(requestedActive) {
		return
	}
	standbyID := requestedStandby.ValueString()
	if requestedStandby.IsUnknown() {
		standbyID = "(known after apply)"
	}
	diags.AddWarning("Virtual-cluster active cluster will change", fmt.Sprintf("Current roles: active %q, standby %q.\nRequested roles: active %q, standby %q.\n\nThis update will change the active cluster from %q to %q. Review the requested pair in the Terraform plan.", active.ValueString(), standby.ValueString(), requestedActive.ValueString(), standbyID, active.ValueString(), requestedActive.ValueString()))
}

func validateVirtualClusterInputs(plan VirtualClusterResourceModel, diags *diag.Diagnostics) {
	for _, value := range []types.String{plan.WarehouseID, plan.Name, plan.DesiredActiveClusterID, plan.DesiredStandbyClusterID} {
		if value.IsNull() || value.IsUnknown() || strings.TrimSpace(value.ValueString()) == "" {
			diags.AddError("Invalid virtual-cluster configuration", "Provide a warehouse ID, name, and two distinct physical-cluster IDs.")
			return
		}
	}
	if plan.DesiredActiveClusterID.Equal(plan.DesiredStandbyClusterID) || plan.PairReconcileTrigger.IsUnknown() {
		diags.AddError("Invalid virtual-cluster configuration", "Member IDs must be distinct, and pair_reconcile_trigger must be known when applying.")
	}
}

func (r *VirtualClusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *VirtualClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan VirtualClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The VeloDB client is not available during creation.")
		return
	}
	validateVirtualClusterInputs(plan, &resp.Diagnostics)
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
	result, err := r.client.CreateVirtualCluster(ctx, warehouseID, &client.CreateVirtualClusterRequest{
		Name: plan.Name.ValueString(), ActiveClusterID: plan.DesiredActiveClusterID.ValueString(),
		StandbyClusterID: plan.DesiredStandbyClusterID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating virtual cluster", fmt.Sprintf(
			"%v\n\nIf the request may have been accepted, inspect virtual clusters in warehouse %q for name %q and the requested members. "+
				"Import the confirmed match with `terraform import <resource_address> %s/<virtual_cluster_id>` before retrying creation. "+
				"If no match is visible, investigate the request ID with support; do not retry creation blindly.",
			err, warehouseID, plan.Name.ValueString(), warehouseID))
		return
	}

	plan.ID = types.StringValue(result.VirtualClusterID)
	// A known ID must be saved without an error diagnostic: Terraform taints failed creates.
	plan.ActiveClusterID, plan.StandbyClusterID, plan.Status = types.StringNull(), types.StringNull(), types.StringNull()
	plan.AtRisk, plan.CreatedAt, plan.UpdatedAt = types.BoolNull(), types.StringNull(), types.StringNull()
	plan.RiskReasons, diags = types.SetValueFrom(ctx, types.StringType, []string{})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cluster, waitErr := waitForVirtualCluster(ctx, func(ctx context.Context) (*client.VirtualCluster, error) {
		return r.client.GetVirtualCluster(ctx, warehouseID, result.VirtualClusterID)
	}, func(cluster *client.VirtualCluster) bool {
		return cluster.ActiveClusterID == plan.DesiredActiveClusterID.ValueString() && cluster.StandbyClusterID == plan.DesiredStandbyClusterID.ValueString()
	}, true, 15*time.Second)
	if cluster != nil {
		resp.Diagnostics.Append(plan.setVirtualClusterObserved(ctx, cluster)...)
		if cluster.Name != plan.Name.ValueString() {
			resp.Diagnostics.AddWarning("Virtual-cluster name differs from request", "The API returned a different name. Refresh Terraform state before making another change.")
		}
	}
	if waitErr != nil {
		resp.Diagnostics.AddWarning("Virtual-cluster creation unresolved", fmt.Sprintf("Virtual cluster %q has been saved in Terraform state. %v. Inspect its status and refresh before another change; do not remove it from state or retry creation blindly.", result.VirtualClusterID, waitErr))
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func waitForVirtualCluster(ctx context.Context, get func(context.Context) (*client.VirtualCluster, error), matches func(*client.VirtualCluster) bool, allowMissing bool, interval time.Duration) (*client.VirtualCluster, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var latest *client.VirtualCluster
	for {
		cluster, err := get(ctx)
		if err != nil {
			if !allowMissing || !virtualClusterNotFound(err) {
				return latest, err
			}
		} else {
			latest = cluster
			if cluster.Status == "CreateFailed" || cluster.Status == "Deleting" || cluster.Status == "Deleted" {
				return latest, fmt.Errorf("API reported %s", cluster.Status)
			}
			if virtualClusterStable(cluster) && matches(cluster) {
				return latest, nil
			}
		}
		select {
		case <-ctx.Done():
			if latest == nil {
				return nil, fmt.Errorf("timed out before the virtual cluster became readable: %w", ctx.Err())
			}
			return latest, fmt.Errorf("timed out waiting for the requested state, last status %q, active %q, standby %q: %w", latest.Status, latest.ActiveClusterID, latest.StandbyClusterID, ctx.Err())
		case <-ticker.C:
		}
	}
}

func virtualClusterStable(cluster *client.VirtualCluster) bool {
	return cluster.Status == "Running" || cluster.Status == "AtRisk"
}

func (r *VirtualClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state VirtualClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateVirtualClusterInputs(plan, &resp.Diagnostics)
	if r.client == nil || state.ID.IsNull() || state.ID.IsUnknown() || state.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Cannot update virtual cluster", "The provider client or existing virtual-cluster ID is missing.")
	}
	if !plan.WarehouseID.Equal(state.WarehouseID) {
		resp.Diagnostics.AddError("Virtual-cluster warehouse cannot be changed", "Restore the original warehouse ID and create a separate virtual cluster for migration.")
	}
	if resp.Diagnostics.HasError() {
		return
	}
	updateTimeout, diags := plan.Timeouts.Update(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Save verified partial progress on errors, retaining the old intent for an unverified pair change.
	stateCtx := ctx
	defer func() { resp.Diagnostics.Append(resp.State.Set(stateCtx, &state)...) }()
	renameChanged := !plan.Name.Equal(state.Name)
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	get := func(ctx context.Context) (*client.VirtualCluster, error) {
		return r.client.GetVirtualCluster(ctx, state.WarehouseID.ValueString(), state.ID.ValueString())
	}
	observe := func(cluster *client.VirtualCluster) {
		if cluster != nil {
			if cluster.Name == "" {
				resp.Diagnostics.AddError("Invalid virtual-cluster response", "The API returned an empty virtual-cluster name.")
				return
			}
			state.Name = types.StringValue(cluster.Name)
			resp.Diagnostics.Append(state.setVirtualClusterObserved(ctx, cluster)...)
		}
	}
	fail := func(action string, err error) {
		if ctx.Err() == nil {
			if latest, readErr := get(ctx); readErr == nil {
				observe(latest)
			}
		}
		title, detail := userError(action, err)
		resp.Diagnostics.AddError(title, detail+"\n\nThe existing virtual-cluster ID remains in state. Refresh and inspect the live name, roles, and status before retrying; a failed request does not prove rollback.")
	}
	current, err := get(ctx)
	if err != nil {
		resp.Diagnostics.AddError(userError("reading virtual cluster before update", err))
		return
	}
	observe(current)
	if resp.Diagnostics.HasError() {
		return
	}
	if !virtualClusterStable(current) {
		resp.Diagnostics.AddError("Virtual cluster is not ready for update", fmt.Sprintf("Current status is %q. Wait for Running or AtRisk and refresh before applying another change.", current.Status))
		return
	}
	if !renameChanged && current.Name != plan.Name.ValueString() {
		resp.Diagnostics.AddError("Virtual-cluster name changed since planning", "Refresh Terraform state and review a new plan before applying another change.")
		return
	}
	triggerChanged := !plan.PairReconcileTrigger.Equal(state.PairReconcileTrigger)
	pairChanged := virtualClusterPairChanged(plan, state)
	if current.Name != plan.Name.ValueString() {
		if err := r.client.RenameVirtualCluster(ctx, state.WarehouseID.ValueString(), state.ID.ValueString(), &client.RenameVirtualClusterRequest{NewName: plan.Name.ValueString()}); err != nil {
			fail("renaming virtual cluster", err)
			return
		}
		current, err = waitForVirtualCluster(ctx, get, func(cluster *client.VirtualCluster) bool {
			return cluster.Name == plan.Name.ValueString()
		}, false, 15*time.Second)
		observe(current)
		if err != nil {
			fail("verifying virtual-cluster rename", err)
			return
		}
		if resp.Diagnostics.HasError() {
			return
		}
	}
	if pairChanged {
		matchesPair := func(cluster *client.VirtualCluster) bool {
			return cluster.ActiveClusterID == plan.DesiredActiveClusterID.ValueString() && cluster.StandbyClusterID == plan.DesiredStandbyClusterID.ValueString()
		}
		if triggerChanged || !matchesPair(current) {
			warnVirtualClusterActiveChange(&resp.Diagnostics, types.StringValue(current.ActiveClusterID), types.StringValue(current.StandbyClusterID), plan.DesiredActiveClusterID, plan.DesiredStandbyClusterID)
			if err := r.client.UpdateVirtualCluster(ctx, state.WarehouseID.ValueString(), state.ID.ValueString(), &client.UpdateVirtualClusterRequest{
				NewActiveClusterID: plan.DesiredActiveClusterID.ValueString(), NewStandbyClusterID: plan.DesiredStandbyClusterID.ValueString(),
			}); err != nil {
				fail("updating virtual-cluster members", err)
				return
			}
			current, err = waitForVirtualCluster(ctx, get, matchesPair, false, 15*time.Second)
			observe(current)
			if err != nil {
				fail("verifying virtual-cluster members", err)
				return
			}
			if resp.Diagnostics.HasError() {
				return
			}
		}
		state.DesiredActiveClusterID, state.DesiredStandbyClusterID = plan.DesiredActiveClusterID, plan.DesiredStandbyClusterID
		state.PairReconcileTrigger = plan.PairReconcileTrigger
	}
	if !state.Name.Equal(plan.Name) {
		resp.Diagnostics.AddError("Virtual-cluster name changed during update", "Refresh Terraform state and review a new plan before retrying.")
		return
	}
	state.Timeouts = plan.Timeouts
}

func (r *VirtualClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state VirtualClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil || state.ID.IsNull() || state.ID.IsUnknown() || strings.TrimSpace(state.ID.ValueString()) == "" ||
		state.WarehouseID.IsNull() || state.WarehouseID.IsUnknown() || strings.TrimSpace(state.WarehouseID.ValueString()) == "" {
		resp.Diagnostics.AddError("Cannot delete virtual cluster", "The provider client, warehouse ID, or virtual-cluster ID is missing.")
		return
	}
	deleteTimeout, diags := state.Timeouts.Delete(ctx, 15*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()
	get := func(ctx context.Context) (*client.VirtualCluster, error) {
		return r.client.GetVirtualCluster(ctx, state.WarehouseID.ValueString(), state.ID.ValueString())
	}
	fail := func(err error) {
		title, detail := userError("confirming virtual-cluster deletion", err)
		resp.Diagnostics.AddError(title, detail+fmt.Sprintf("\n\nDeletion of virtual cluster %q is not confirmed. Its ID remains in Terraform state. Inspect the backend deletion task and refresh before retrying; do not assume routing or automatic failover was restored after a failed deletion.", state.ID.ValueString()))
	}
	current, err := get(ctx)
	if virtualClusterNotFound(err) {
		if virtualClusterCreationUnresolved(state) {
			fail(fmt.Errorf("GET reports not found while the previously accepted create remains unresolved; delayed visibility or a hidden failed create does not prove cleanup completed"))
			return
		}
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		fail(err)
		return
	}
	if current.Status != "Deleted" {
		if current.Status != "Deleting" && !virtualClusterStable(current) {
			resp.Diagnostics.AddError("Virtual cluster is not ready for deletion", fmt.Sprintf("Current status is %q. Inspect the backend operation and restore a deletable state before retrying. The virtual-cluster ID remains in Terraform state.", current.Status))
			return
		}
		// Repeated DELETE acknowledges an active task or retries failed cleanup on the backend.
		if deleteErr := r.client.DeleteVirtualCluster(ctx, state.WarehouseID.ValueString(), state.ID.ValueString()); deleteErr != nil {
			// A lost response or concurrent delete may have started the task; verify without resubmitting.
			latest, readErr := get(ctx)
			if virtualClusterNotFound(readErr) {
				resp.State.RemoveResource(ctx)
				return
			}
			if readErr != nil {
				fail(fmt.Errorf("delete request: %v; verifying target: %w", deleteErr, readErr))
				return
			}
			var apiErr *client.APIError
			pending := errors.As(deleteErr, &apiErr) && apiErr.StatusCode == http.StatusAccepted
			if !pending && latest.Status != "Deleting" && latest.Status != "Deleted" {
				fail(deleteErr)
				return
			}
		}
	}
	if err := waitForVirtualClusterAbsent(ctx, get, 15*time.Second); err != nil {
		fail(err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func virtualClusterNotFound(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.IsVirtualClusterNotFound()
}

func virtualClusterCreationUnresolved(state VirtualClusterResourceModel) bool {
	return state.Status.IsNull() || state.Status.IsUnknown() || state.Status.ValueString() == "Creating" || state.Status.ValueString() == "CreateFailed"
}

func waitForVirtualClusterAbsent(ctx context.Context, get func(context.Context) (*client.VirtualCluster, error), interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		cluster, err := get(ctx)
		if virtualClusterNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if cluster == nil {
			return fmt.Errorf("empty response does not confirm virtual-cluster absence")
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for virtual-cluster absence; last status %q: %w", cluster.Status, ctx.Err())
		case <-ticker.C:
		}
	}
}

func (r *VirtualClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state VirtualClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil || state.ID.IsNull() || state.ID.IsUnknown() || state.ID.ValueString() == "" ||
		state.WarehouseID.IsNull() || state.WarehouseID.IsUnknown() || state.WarehouseID.ValueString() == "" {
		resp.Diagnostics.AddError("Cannot read virtual cluster", "The provider client, warehouse ID, or virtual-cluster ID is missing.")
		return
	}

	cluster, err := r.client.GetVirtualCluster(ctx, state.WarehouseID.ValueString(), state.ID.ValueString())
	if err != nil {
		if virtualClusterNotFound(err) {
			if virtualClusterCreationUnresolved(state) {
				resp.Diagnostics.AddError("Virtual-cluster creation outcome uncertain", fmt.Sprintf("Virtual cluster %q was previously accepted but is not currently readable. Verify its backend state before removing it from Terraform state or retrying creation.", state.ID.ValueString()))
				return
			}
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(userError("reading virtual cluster", err))
		return
	}
	if (state.DesiredActiveClusterID.IsNull() || state.DesiredActiveClusterID.IsUnknown() ||
		state.DesiredStandbyClusterID.IsNull() || state.DesiredStandbyClusterID.IsUnknown()) &&
		(cluster.ActiveClusterID == "" || cluster.StandbyClusterID == "") {
		resp.Diagnostics.AddError("Cannot initialize imported virtual cluster", "The API did not return both member IDs; retry after the virtual cluster is ready.")
		return
	}
	if cluster.Status == "CreateFailed" {
		resp.Diagnostics.AddError("Virtual-cluster creation failed", "The API reports CreateFailed for this virtual cluster. Inspect and repair it before another Terraform change.")
		return
	}
	if cluster.Name == "" {
		resp.Diagnostics.AddError("Invalid virtual-cluster response", "The API returned an empty virtual-cluster name.")
		return
	}
	priorActive, priorStandby := state.ActiveClusterID.ValueString(), state.StandbyClusterID.ValueString()
	if priorActive != "" && priorStandby != "" && priorActive != priorStandby && cluster.ActiveClusterID == priorStandby && cluster.StandbyClusterID == priorActive {
		resp.Diagnostics.AddWarning("Virtual-cluster roles switched", fmt.Sprintf("Roles switched since the last refresh:\nActive: %q -> %q\nStandby: %q -> %q\n\nThe configured desired pair is unchanged. Review the current roles before changing members. The API does not identify the cause of the switch.", priorActive, cluster.ActiveClusterID, priorStandby, cluster.StandbyClusterID))
	}

	if state.DesiredActiveClusterID.IsNull() || state.DesiredActiveClusterID.IsUnknown() {
		state.DesiredActiveClusterID = types.StringValue(cluster.ActiveClusterID)
	}
	if state.DesiredStandbyClusterID.IsNull() || state.DesiredStandbyClusterID.IsUnknown() {
		state.DesiredStandbyClusterID = types.StringValue(cluster.StandbyClusterID)
	}
	state.Name = types.StringValue(cluster.Name)
	resp.Diagnostics.Append(state.setVirtualClusterObserved(ctx, cluster)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cluster.Status == "Creating" {
		resp.Diagnostics.AddWarning("Virtual-cluster creation in progress", "The virtual cluster has not reached Running or AtRisk. Check its status before making another change.")
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (state *VirtualClusterResourceModel) setVirtualClusterObserved(ctx context.Context, cluster *client.VirtualCluster) diag.Diagnostics {
	state.ActiveClusterID = stringOrNull(cluster.ActiveClusterID)
	state.StandbyClusterID = stringOrNull(cluster.StandbyClusterID)
	state.Status = stringOrNull(cluster.Status)
	state.AtRisk = types.BoolValue(cluster.AtRisk)
	state.CreatedAt = stringOrNull(cluster.CreatedAt)
	state.UpdatedAt = stringOrNull(cluster.UpdatedAt)
	risks := cluster.RiskReasons
	if risks == nil {
		risks = []string{}
	}
	var diags diag.Diagnostics
	state.RiskReasons, diags = types.SetValueFrom(ctx, types.StringType, risks)
	return diags
}

func (r *VirtualClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" ||
		parts[0] != strings.TrimSpace(parts[0]) || parts[1] != strings.TrimSpace(parts[1]) {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: warehouse_id/virtual_cluster_id")
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The VeloDB client is not available during import.")
		return
	}
	if _, err := r.client.GetVirtualCluster(ctx, parts[0], parts[1]); err != nil {
		resp.Diagnostics.AddError(userError("importing virtual cluster", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("warehouse_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}
