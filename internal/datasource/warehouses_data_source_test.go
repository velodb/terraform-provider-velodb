package datasource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestWarehousesZoneMapRead(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"SaaS", "BYOC"} {
		for _, source := range []string{"list", "detail", "list with enrichment", "detail unavailable"} {
			t.Run(mode+"/"+source, func(t *testing.T) {
				var current map[string]string
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					wh := map[string]any{
						"warehouseId": "WH-001", "name": "test", "cloudProvider": "aws",
						"region": "us-east-1", "zone": "us-east-1d", "deploymentMode": mode,
					}
					if current != nil {
						wh["zoneMap"] = current
					}
					var data any = wh
					switch r.URL.Path {
					case "/v1/warehouses":
						if source == "detail" {
							delete(wh, "zoneMap")
						}
						if source == "list" {
							wh["endpointServiceId"] = "vpce-svc-test"
							wh["endpointServiceName"] = "test-service"
						}
						data = []any{wh}
					case "/v1/warehouses/WH-001":
						if source == "list" {
							t.Error("unexpected detail request")
						}
						if source == "detail unavailable" {
							http.Error(w, "unavailable", http.StatusInternalServerError)
							return
						}
						if source == "list with enrichment" && current != nil {
							wh["zoneMap"] = map[string]string{"other-zone": "Other"}
						}
					default:
						t.Errorf("unexpected request: %s", r.URL.Path)
					}
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data, "total": 1}); err != nil {
						t.Error(err)
					}
				}))
				defer ts.Close()
				d := &WarehousesDataSource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "test-key", 0, time.Second)}
				var schemaResp datasource.SchemaResponse
				d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
				tfType := schemaResp.Schema.Type().TerraformType(ctx)
				attrs := make(map[string]tftypes.Value)
				for name, typ := range tfType.(tftypes.Object).AttributeTypes {
					attrs[name] = tftypes.NewValue(typ, nil)
				}
				config := tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tfType, attrs)}
				state := tfsdk.State{Schema: schemaResp.Schema}
				for _, zones := range []map[string]string{
					{"us-east-1d": "Primary", "us-east-1a": "Secondary"},
					{"us-east-1b": "Updated"},
					{},
					nil,
				} {
					current = zones
					resp := datasource.ReadResponse{State: state}
					d.Read(ctx, datasource.ReadRequest{Config: config}, &resp)
					if resp.Diagnostics.HasError() {
						t.Fatal(resp.Diagnostics)
					}
					var got types.Map
					if diags := resp.State.GetAttribute(ctx, path.Root("warehouses").AtListIndex(0).AtName("zone_map"), &got); diags.HasError() {
						t.Fatal(diags)
					}
					want, diags := types.MapValueFrom(ctx, types.StringType, zones)
					if diags.HasError() || !got.Equal(want) {
						t.Fatalf("zone_map = %s, want %s; diagnostics: %v", got, want, diags)
					}
					var primary string
					if diags := resp.State.GetAttribute(ctx, path.Root("warehouses").AtListIndex(0).AtName("zone"), &primary); diags.HasError() || primary != "us-east-1d" {
						t.Fatalf("primary zone = %q; diagnostics: %v", primary, diags)
					}
					state = resp.State
				}
			})
		}
	}
}
