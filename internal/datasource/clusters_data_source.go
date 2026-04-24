package datasource

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"strings"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

var _ datasource.DataSource = &ClustersDataSource{}

type ClustersDataSource struct {
	client *client.FormationClient
}

func NewClustersDataSource() datasource.DataSource {
	return &ClustersDataSource{}
}

type ClustersDataSourceModel struct {
	WarehouseID types.String `tfsdk:"warehouse_id"`
	Keyword     types.String `tfsdk:"keyword"`
	Status      types.String `tfsdk:"status"`
	ClusterType types.String `tfsdk:"cluster_type"`
	BillingModel types.String `tfsdk:"billing_model"`
	Clusters    types.List   `tfsdk:"clusters"`
	Total       types.Int64  `tfsdk:"total"`
}

func (d *ClustersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_clusters"
}

func (d *ClustersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List VeloDB Cloud clusters within a warehouse.",
		Attributes: map[string]schema.Attribute{
			"warehouse_id": schema.StringAttribute{
				Description: "Parent warehouse identifier.",
				Required:    true,
			},
			"keyword": schema.StringAttribute{
				Description: "Fuzzy match on cluster name or ID.",
				Optional:    true,
			},
			"status": schema.StringAttribute{
				Description: "Cluster status filter.",
				Optional:    true,
			},
			"cluster_type": schema.StringAttribute{
				Description: "Cluster type filter: SQL, COMPUTE, or OBSERVER.",
				Optional:    true,
			},
			"billing_model": schema.StringAttribute{
				Description: "Pay type filter: PostPaid or PrePaid.",
				Optional:    true,
			},
			"total": schema.Int64Attribute{
				Description: "Total number of matching clusters.",
				Computed:    true,
			},
			"clusters": schema.ListNestedAttribute{
				Description: "List of clusters.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"cluster_id":     schema.StringAttribute{Computed: true},
						"warehouse_id":   schema.StringAttribute{Computed: true},
						"name":           schema.StringAttribute{Computed: true},
						"status":         schema.StringAttribute{Computed: true},
						"cluster_type":   schema.StringAttribute{Computed: true},
						"cloud_provider": schema.StringAttribute{Computed: true},
						"region":         schema.StringAttribute{Computed: true},
						"zone":           schema.StringAttribute{Computed: true},
						"disk_sum_size":  schema.Int64Attribute{Computed: true},
						"billing_model":       schema.StringAttribute{Computed: true},
						"created_at":     schema.StringAttribute{Computed: true},
						"started_at":     schema.StringAttribute{Computed: true},
						"expire_time":    schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ClustersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.FormationClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.FormationClient, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *ClustersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ClustersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := &client.ListClustersOptions{
		Page: 1,
		Size: 100,
	}
	if !config.Keyword.IsNull() {
		opts.Keyword = config.Keyword.ValueString()
	}
	if !config.Status.IsNull() {
		opts.Status = config.Status.ValueString()
	}
	if !config.ClusterType.IsNull() {
		opts.ClusterType = config.ClusterType.ValueString()
	}
	if !config.BillingModel.IsNull() {
		opts.BillingModel = config.BillingModel.ValueString()
	}

	result, err := d.client.ListClusters(ctx, config.WarehouseID.ValueString(), opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing clusters", err.Error())
		return
	}

	clusterAttrTypes := map[string]attr.Type{
		"cluster_id":     types.StringType,
		"warehouse_id":   types.StringType,
		"name":           types.StringType,
		"status":         types.StringType,
		"cluster_type":   types.StringType,
		"cloud_provider": types.StringType,
		"region":         types.StringType,
		"zone":           types.StringType,
		"disk_sum_size":  types.Int64Type,
		"billing_model":       types.StringType,
		"created_at":     types.StringType,
		"started_at":     types.StringType,
		"expire_time":    types.StringType,
	}

	// Filter out internal/system clusters (e.g., "meta" cluster with m- prefix)
	var filtered []client.ClusterItem
	for _, cl := range result.Data {
		if strings.HasPrefix(cl.ClusterID, "m-") {
			continue
		}
		filtered = append(filtered, cl)
	}

	var items []attr.Value
	for _, cl := range filtered {
		createdAt := types.StringNull()
		if cl.CreatedAt != nil {
			createdAt = types.StringValue(cl.CreatedAt.Format(time.RFC3339))
		}
		startedAt := types.StringNull()
		if cl.StartedAt != nil {
			startedAt = types.StringValue(cl.StartedAt.Format(time.RFC3339))
		}
		expireTime := types.StringNull()
		if cl.ExpireTime != nil {
			expireTime = types.StringValue(cl.ExpireTime.Format(time.RFC3339))
		}

		diskSize := types.Int64Null()
		if cl.DiskSumSize > 0 {
			diskSize = types.Int64Value(int64(cl.DiskSumSize))
		}

		obj, diags := types.ObjectValue(clusterAttrTypes, map[string]attr.Value{
			"cluster_id":     types.StringValue(cl.ClusterID),
			"warehouse_id":   types.StringValue(cl.WarehouseID),
			"name":           types.StringValue(cl.Name),
			"status":         types.StringValue(cl.Status),
			"cluster_type":   stringVal(cl.ClusterType),
			"cloud_provider": stringVal(cl.CloudProvider),
			"region":         stringVal(cl.Region),
			"zone":           stringVal(cl.Zone),
			"disk_sum_size":  diskSize,
			"billing_model":       stringVal(cl.BillingModel),
			"created_at":     createdAt,
			"started_at":     startedAt,
			"expire_time":    expireTime,
		})
		resp.Diagnostics.Append(diags...)
		items = append(items, obj)
	}

	list, diags := types.ListValue(types.ObjectType{AttrTypes: clusterAttrTypes}, items)
	resp.Diagnostics.Append(diags...)

	config.Clusters = list
	config.Total = types.Int64Value(int64(len(filtered)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
