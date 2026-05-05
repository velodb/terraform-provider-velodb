package provider

import (
	"context"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
	velodb_datasource "github.com/velodb/terraform-provider-velodb/internal/datasource"
	velodb_resource "github.com/velodb/terraform-provider-velodb/internal/resource"
)

var _ provider.Provider = &VeloDBProvider{}

type VeloDBProvider struct {
	version string
}

type VeloDBProviderModel struct {
	Host    types.String `tfsdk:"host"`
	APIKey  types.String `tfsdk:"api_key"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &VeloDBProvider{version: version}
	}
}

func (p *VeloDBProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "velodb"
	resp.Version = p.version
}

func (p *VeloDBProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for VeloDB Cloud (Formation OpenAPI).",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Description: "Formation API host (e.g., api.selectdbcloud.com). Can also be set via VELODB_HOST env var.",
				Optional:    true,
			},
			"api_key": schema.StringAttribute{
				Description: "API key for authentication. Can also be set via VELODB_API_KEY env var.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *VeloDBProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config VeloDBProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	host := stringValueOrEnv(config.Host, "VELODB_HOST")
	apiKey := stringValueOrEnv(config.APIKey, "VELODB_API_KEY")

	if host == "" {
		resp.Diagnostics.AddError("Missing host", "Set 'host' in provider block or VELODB_HOST environment variable.")
		return
	}
	if apiKey == "" {
		resp.Diagnostics.AddError("Missing api_key", "Set 'api_key' in provider block or VELODB_API_KEY environment variable.")
		return
	}

	c := client.NewFormationClient(host, apiKey, 3, 60*time.Second)

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *VeloDBProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		velodb_resource.NewWarehouseResource,
		velodb_resource.NewClusterResource,
		velodb_resource.NewPublicAccessPolicyResource,
		velodb_resource.NewPrivateLinkEndpointServiceResource,
		velodb_resource.NewWarehousePrivateEndpointResource,
	}
}

func (p *VeloDBProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		velodb_datasource.NewWarehousesDataSource,
		velodb_datasource.NewClustersDataSource,
		velodb_datasource.NewWarehouseConnectionsDataSource,
		velodb_datasource.NewWarehouseVersionsDataSource,
	}
}

func stringValueOrEnv(val types.String, envKey string) string {
	if !val.IsNull() && !val.IsUnknown() {
		return val.ValueString()
	}
	return os.Getenv(envKey)
}
