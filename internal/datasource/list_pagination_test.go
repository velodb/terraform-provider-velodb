package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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

// pagedServer serves items for listPath one page at a time, honoring the
// page and size query parameters and reporting the full total.
func pagedServer(t *testing.T, listPath string, items []map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != listPath {
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		size, _ := strconv.Atoi(r.URL.Query().Get("size"))
		if page < 1 || size < 1 {
			t.Errorf("request without page/size: %s", r.URL.RawQuery)
			return
		}
		start := min((page-1)*size, len(items))
		end := min(start+size, len(items))
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"success": true, "data": items[start:end], "page": page, "size": size, "total": len(items),
		}); err != nil {
			t.Error(err)
		}
	}))
}

func readDataSource(t *testing.T, d datasource.DataSource, values map[string]tftypes.Value) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	tfType := schemaResp.Schema.Type().TerraformType(ctx)
	attrs := make(map[string]tftypes.Value)
	for name, typ := range tfType.(tftypes.Object).AttributeTypes {
		attrs[name] = tftypes.NewValue(typ, nil)
	}
	for name, v := range values {
		attrs[name] = v
	}
	config := tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(tfType, attrs)}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	d.Read(ctx, datasource.ReadRequest{Config: config}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	return resp.State
}

func listLenAndTotal(t *testing.T, state tfsdk.State, listAttr string) (int, int64) {
	t.Helper()
	ctx := context.Background()
	var list types.List
	var total int64
	if diags := state.GetAttribute(ctx, path.Root(listAttr), &list); diags.HasError() {
		t.Fatal(diags)
	}
	if diags := state.GetAttribute(ctx, path.Root("total"), &total); diags.HasError() {
		t.Fatal(diags)
	}
	return len(list.Elements()), total
}

func newTestClient(ts *httptest.Server) *client.FormationClient {
	return client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "test-key", 0, time.Second)
}

func TestWarehousesDataSourceReadsEveryPage(t *testing.T) {
	var items []map[string]any
	for i := 1; i <= 2*listPageSize+5; i++ {
		items = append(items, map[string]any{
			"warehouseId": fmt.Sprintf("WH-%03d", i), "name": fmt.Sprintf("wh-%03d", i),
			"cloudProvider": "aws", "region": "us-east-1",
			"endpointServiceId": "vpce-svc", "endpointServiceName": "svc",
		})
	}
	items[len(items)-1]["name"] = "analytics-prod"
	ts := pagedServer(t, "/v1/warehouses", items)
	defer ts.Close()
	d := &WarehousesDataSource{client: newTestClient(ts)}

	n, total := listLenAndTotal(t, readDataSource(t, d, nil), "warehouses")
	if n != len(items) || total != int64(len(items)) {
		t.Fatalf("got %d warehouses (total %d), want %d", n, total, len(items))
	}

	// keyword filtering happens client-side, so it must see every page
	n, total = listLenAndTotal(t, readDataSource(t, d, map[string]tftypes.Value{
		"keyword": tftypes.NewValue(tftypes.String, "analytics"),
	}), "warehouses")
	if n != 1 || total != 1 {
		t.Fatalf("keyword match on the last page: got %d warehouses (total %d), want 1", n, total)
	}
}

func TestClustersDataSourceReadsEveryPage(t *testing.T) {
	var items []map[string]any
	for i := 1; i <= listPageSize+1; i++ {
		items = append(items, map[string]any{
			"clusterId": fmt.Sprintf("c-%03d", i), "warehouseId": "WH-001",
			"name": fmt.Sprintf("cluster-%03d", i), "status": "Running", "clusterType": "COMPUTE",
		})
	}
	items[len(items)-1]["name"] = "etl-nightly"
	ts := pagedServer(t, "/v1/warehouses/WH-001/clusters", items)
	defer ts.Close()
	d := &ClustersDataSource{client: newTestClient(ts)}
	warehouse := map[string]tftypes.Value{"warehouse_id": tftypes.NewValue(tftypes.String, "WH-001")}

	n, total := listLenAndTotal(t, readDataSource(t, d, warehouse), "clusters")
	if n != len(items) || total != int64(len(items)) {
		t.Fatalf("got %d clusters (total %d), want %d", n, total, len(items))
	}

	warehouse["keyword"] = tftypes.NewValue(tftypes.String, "etl")
	n, total = listLenAndTotal(t, readDataSource(t, d, warehouse), "clusters")
	if n != 1 || total != 1 {
		t.Fatalf("keyword match on the last page: got %d clusters (total %d), want 1", n, total)
	}
}
