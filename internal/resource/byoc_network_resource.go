package resource

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

var (
	_ resource.Resource                   = &BYOCNetworkResource{}
	_ resource.ResourceWithImportState    = &BYOCNetworkResource{}
	_ resource.ResourceWithValidateConfig = &BYOCNetworkResource{}
)

type BYOCNetworkResource struct {
	client *client.FormationClient
}

func NewBYOCNetworkResource() resource.Resource {
	return &BYOCNetworkResource{}
}

type BYOCNetworkResourceModel struct {
	ID              types.Int64  `tfsdk:"id"`
	CloudProvider   types.String `tfsdk:"cloud_provider"`
	Name            types.String `tfsdk:"name"`
	CredentialID    types.Int64  `tfsdk:"credential_id"`
	ZoneMappings    types.List   `tfsdk:"zone_mappings"`
	SecurityGroupID types.String `tfsdk:"security_group_id"`
	EndpointID      types.String `tfsdk:"endpoint_id"`
	Region          types.String `tfsdk:"region"`
	VPCID           types.String `tfsdk:"vpc_id"`
	WarehouseCount  types.Int64  `tfsdk:"warehouse_count"`
	WarehouseIDs    types.List   `tfsdk:"warehouse_ids"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
}

type BYOCZoneMappingModel struct {
	ZoneID   types.String `tfsdk:"zone_id"`
	SubnetID types.String `tfsdk:"subnet_id"`
}

func (r *BYOCNetworkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_byoc_network"
}

func (r *BYOCNetworkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "Registers an AWS VPC network configuration for an advanced BYOC warehouse.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "VeloDB network configuration ID.",
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
				Description:   "Network configuration name.",
				Required:      true,
				Validators:    []validator.String{stringvalidator.LengthBetween(1, 32)},
				PlanModifiers: replace,
			},
			"credential_id": schema.Int64Attribute{
				Description: "Credential configuration ID used by this network.",
				Required:    true,
				Validators:  []validator.Int64{int64validator.AtLeast(1)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"zone_mappings": schema.ListNestedAttribute{
				Description: "One mapping for single-zone deployment or three unique mappings for cross-zone deployment.",
				Required:    true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"zone_id": schema.StringAttribute{
						Description: "AWS Availability Zone name, for example us-east-1a.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
					},
					"subnet_id": schema.StringAttribute{
						Description: "Private subnet ID in this Availability Zone.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
					},
				}},
			},
			"security_group_id": schema.StringAttribute{
				Description:   "AWS security group ID for the warehouse.",
				Required:      true,
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: replace,
			},
			"endpoint_id": schema.StringAttribute{
				Description:   "Optional AWS VPC endpoint ID.",
				Optional:      true,
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: replace,
			},
			"region":          schema.StringAttribute{Description: "AWS region derived from the registered network.", Computed: true},
			"vpc_id":          schema.StringAttribute{Description: "AWS VPC ID derived from the registered network.", Computed: true},
			"warehouse_count": schema.Int64Attribute{Description: "Number of warehouses using this network configuration.", Computed: true},
			"warehouse_ids": schema.ListAttribute{
				Description: "Warehouses using this network configuration.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"created_at": schema.StringAttribute{Description: "Creation time in RFC 3339 format.", Computed: true},
			"updated_at": schema.StringAttribute{Description: "Last update time in RFC 3339 format.", Computed: true},
		},
	}
}

func (r *BYOCNetworkResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var mappings types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("zone_mappings"), &mappings)...)
	if resp.Diagnostics.HasError() || mappings.IsNull() || mappings.IsUnknown() {
		return
	}

	var values []BYOCZoneMappingModel
	resp.Diagnostics.Append(mappings.ElementsAs(ctx, &values, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(values) != 1 && len(values) != 3 {
		resp.Diagnostics.AddAttributeError(path.Root("zone_mappings"), "Invalid number of zone mappings", "Provide exactly one zone mapping for single-zone deployment or exactly three for cross-zone deployment.")
		return
	}

	zones := make(map[string]struct{}, len(values))
	subnets := make(map[string]struct{}, len(values))
	for i, mapping := range values {
		if mapping.ZoneID.IsNull() || mapping.ZoneID.IsUnknown() || mapping.SubnetID.IsNull() || mapping.SubnetID.IsUnknown() {
			continue
		}
		zone := mapping.ZoneID.ValueString()
		subnet := mapping.SubnetID.ValueString()
		if _, exists := zones[zone]; exists {
			resp.Diagnostics.AddAttributeError(path.Root("zone_mappings").AtListIndex(i).AtName("zone_id"), "Duplicate Availability Zone", fmt.Sprintf("Availability Zone %q is used more than once.", zone))
		}
		if _, exists := subnets[subnet]; exists {
			resp.Diagnostics.AddAttributeError(path.Root("zone_mappings").AtListIndex(i).AtName("subnet_id"), "Duplicate subnet", fmt.Sprintf("Subnet %q is used more than once.", subnet))
		}
		zones[zone] = struct{}{}
		subnets[subnet] = struct{}{}
	}
}

func (r *BYOCNetworkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BYOCNetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BYOCNetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mappings := expandBYOCZoneMappings(ctx, plan.ZoneMappings, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := &client.CreateCloudSettingNetworkConfigRequest{
		Name:            plan.Name.ValueString(),
		CredentialID:    plan.CredentialID.ValueInt64(),
		ZoneMappings:    mappings,
		SecurityGroupID: plan.SecurityGroupID.ValueString(),
	}
	setOptionalString(&apiReq.EndpointID, plan.EndpointID)
	result, err := r.client.CreateCloudSettingNetworkConfig(ctx, plan.CloudProvider.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError(userError("creating BYOC network configuration", err))
		return
	}

	plan.ID = types.Int64Value(result.NetworkConfigID)
	found, err := r.read(ctx, &plan, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(userError("reading BYOC network configuration after creation", err))
		return
	}
	if !found {
		resp.Diagnostics.AddError("Network configuration disappeared after creation", "VeloDB created the network configuration but it could not be read back.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BYOCNetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BYOCNetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, err := r.read(ctx, &state, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(userError("reading BYOC network configuration", err))
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BYOCNetworkResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("BYOC network configuration cannot be updated", "Change to this resource must replace it because the VeloDB API has no update operation.")
}

func (r *BYOCNetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BYOCNetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteCloudSettingNetworkConfig(ctx, state.CloudProvider.ValueString(), state.ID.ValueInt64()); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError(userError("deleting BYOC network configuration", err))
	}
}

func (r *BYOCNetworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	cloudProvider, id, credentialID, err := parseBYOCNetworkImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The VeloDB client is not available during import.")
		return
	}
	if _, err := r.client.GetCloudSettingNetworkConfig(ctx, cloudProvider, id); err != nil {
		resp.Diagnostics.AddError(userError("importing BYOC network configuration", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cloud_provider"), cloudProvider)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("credential_id"), credentialID)...)
}

func (r *BYOCNetworkResource) read(ctx context.Context, state *BYOCNetworkResourceModel, diags *diag.Diagnostics) (bool, error) {
	item, err := r.client.GetCloudSettingNetworkConfig(ctx, state.CloudProvider.ValueString(), state.ID.ValueInt64())
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return false, nil
		}
		return false, err
	}

	state.ID = types.Int64Value(item.NetworkConfigID)
	state.CloudProvider = types.StringValue(item.CloudProvider)
	state.Name = types.StringValue(item.Name)
	state.ZoneMappings = flattenBYOCZoneMappings(ctx, item.ZoneMappings, diags)
	state.SecurityGroupID = types.StringValue(item.SecurityGroupID)
	state.EndpointID = stringOrNull(item.EndpointID)
	state.Region = stringOrNull(item.Region)
	state.VPCID = stringOrNull(item.VPCID)
	state.WarehouseCount = types.Int64Value(int64(item.WarehouseCount))
	warehouseIDs, warehouseDiags := types.ListValueFrom(ctx, types.StringType, item.WarehouseIDs)
	diags.Append(warehouseDiags...)
	state.WarehouseIDs = warehouseIDs
	state.CreatedAt = timeOrNull(item.CreatedAt)
	state.UpdatedAt = timeOrNull(item.UpdatedAt)
	return true, nil
}

func expandBYOCZoneMappings(ctx context.Context, mappings types.List, diags *diag.Diagnostics) []client.CloudSettingZoneMapping {
	var values []BYOCZoneMappingModel
	diags.Append(mappings.ElementsAs(ctx, &values, false)...)
	result := make([]client.CloudSettingZoneMapping, 0, len(values))
	for _, mapping := range values {
		result = append(result, client.CloudSettingZoneMapping{ZoneID: mapping.ZoneID.ValueString(), SubnetID: mapping.SubnetID.ValueString()})
	}
	return result
}

func flattenBYOCZoneMappings(ctx context.Context, mappings []client.CloudSettingZoneMapping, diags *diag.Diagnostics) types.List {
	values := make([]BYOCZoneMappingModel, 0, len(mappings))
	for _, mapping := range mappings {
		values = append(values, BYOCZoneMappingModel{ZoneID: types.StringValue(mapping.ZoneID), SubnetID: types.StringValue(mapping.SubnetID)})
	}
	result, mappingDiags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: byocZoneMappingAttrTypes()}, values)
	diags.Append(mappingDiags...)
	return result
}

func byocZoneMappingAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{"zone_id": types.StringType, "subnet_id": types.StringType}
}

func parseBYOCNetworkImportID(importID string) (string, int64, int64, error) {
	parts := strings.Split(strings.TrimSpace(importID), "/")
	if len(parts) != 3 || parts[0] != "aws" {
		return "", 0, 0, fmt.Errorf("expected format: aws/<network_config_id>/<credential_id>")
	}
	networkID, networkErr := strconv.ParseInt(parts[1], 10, 64)
	credentialID, credentialErr := strconv.ParseInt(parts[2], 10, 64)
	if networkErr != nil || credentialErr != nil || networkID <= 0 || credentialID <= 0 {
		return "", 0, 0, fmt.Errorf("expected format: aws/<positive network_config_id>/<positive credential_id>")
	}
	return parts[0], networkID, credentialID, nil
}
