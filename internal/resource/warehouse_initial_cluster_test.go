package resource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func warehouseIdentityTestState(t *testing.T) tfsdk.State {
	t.Helper()
	var resp resource.SchemaResponse
	(&WarehouseResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	ty := resp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
	values := map[string]tftypes.Value{}
	for name, attrType := range ty.AttributeTypes {
		values[name] = tftypes.NewValue(attrType, nil)
	}
	return tfsdk.State{Schema: resp.Schema, Raw: tftypes.NewValue(ty, values)}
}

func TestWarehouseCreateUsesAuthoritativeIdentity(t *testing.T) {
	for _, mode := range []string{"known", "delayed", "omitted timeout", "pending timeout", "read denied"} {
		t.Run(mode, func(t *testing.T) {
			reads, creates := 0, 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch {
				case req.Method == http.MethodPost && req.URL.Path == "/v1/warehouses":
					creates++
					_ = json.NewEncoder(w).Encode(client.APIResponse[client.CreateWarehouseResult]{Success: true, Data: client.CreateWarehouseResult{WarehouseID: "w-new"}})
				case req.Method == http.MethodGet && req.URL.Path == "/v1/warehouses/w-new":
					reads++
					if mode == "read denied" {
						w.WriteHeader(http.StatusForbidden)
						_ = json.NewEncoder(w).Encode(map[string]any{"code": "Forbidden", "message": "read denied", "requestId": "request-test"})
						return
					}
					data := map[string]any{"warehouseId": "w-new", "status": "Running"}
					switch mode {
					case "known":
						data["initialClusterId"] = "c-new"
					case "delayed":
						data["initialClusterId"] = nil
						if reads >= 2 {
							data["initialClusterId"] = "c-new"
						}
					case "pending timeout":
						data["initialClusterId"] = nil
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
				default:
					t.Errorf("creation must not list physical clusters: %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer ts.Close()
			ctx := context.Background()
			if mode == "pending timeout" || mode == "omitted timeout" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
			}
			state := warehouseIdentityTestState(t)
			plan := tfsdk.Plan(state)
			for name, value := range map[string]any{
				"id": "w-old", "initial_cluster_id": "c-old", "name": "test", "deployment_mode": "SaaS",
				"cloud_provider": "aws", "region": "us-east-1", "admin_password": "TestPass@123",
				"initial_cluster": initialClusterListForTest(types.Int64Null()),
			} {
				if diags := plan.SetAttribute(ctx, path.Root(name), value); diags.HasError() {
					t.Fatal(diags)
				}
			}
			r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
			resp := resource.CreateResponse{State: state}
			r.Create(ctx, resource.CreateRequest{Plan: plan}, &resp)
			var result WarehouseResourceModel
			if diags := resp.State.Get(context.Background(), &result); diags.HasError() {
				t.Fatal(diags)
			}
			want := types.StringNull()
			if mode == "known" || mode == "delayed" {
				want = types.StringValue("c-new")
			}
			wantError := mode == "pending timeout" || mode == "omitted timeout" || mode == "read denied"
			if resp.Diagnostics.HasError() != wantError || result.ID.ValueString() != "w-new" || !result.InitialClusterID.Equal(want) || creates != 1 {
				t.Fatalf("created warehouse must retain only its own identity: id=%s initial=%s diagnostics=%v creates=%d", result.ID, result.InitialClusterID, resp.Diagnostics, creates)
			}
			if mode == "delayed" && reads < 3 {
				t.Fatalf("creation did not wait for initial identity: reads=%d", reads)
			}
		})
	}
}

func TestWarehouseImportReadUpdateUseAPIIdentity(t *testing.T) {
	for _, field := range []string{`"initialClusterId":"c-original",`, `"initialClusterId":null,`, ""} {
		t.Run(field, func(t *testing.T) {
			requests := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests++
				if req.Method != http.MethodGet || req.URL.Path != "/v1/warehouses/w-existing" {
					t.Errorf("import/read/update must not infer identity from clusters or mutate the warehouse: %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				_, _ = w.Write([]byte(`{"success":true,"data":{` + field + `"warehouseId":"w-existing","name":"existing","status":"Running"}}`))
			}))
			defer ts.Close()
			ctx := context.Background()
			r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
			imported := resource.ImportStateResponse{State: warehouseIdentityTestState(t)}
			r.ImportState(ctx, resource.ImportStateRequest{ID: "w-existing"}, &imported)
			known := strings.Contains(field, "c-original")
			if imported.Diagnostics.HasError() || (len(imported.Diagnostics.Warnings()) > 0) == known {
				t.Fatalf("import must warn only when original identity is unknown: %v", imported.Diagnostics)
			}
			read := resource.ReadResponse{State: imported.State}
			r.Read(ctx, resource.ReadRequest{State: imported.State}, &read)
			if read.Diagnostics.HasError() {
				t.Fatal(read.Diagnostics)
			}
			plan := tfsdk.Plan(read.State)
			if diags := plan.SetAttribute(ctx, path.Root("admin_password_version"), int64(1)); diags.HasError() {
				t.Fatal(diags)
			}
			updated := resource.UpdateResponse{State: read.State}
			r.Update(ctx, resource.UpdateRequest{Config: tfsdk.Config(plan), Plan: plan, State: read.State}, &updated)
			var result WarehouseResourceModel
			if diags := updated.State.Get(ctx, &result); diags.HasError() {
				t.Fatal(diags)
			}
			want := types.StringNull()
			if known {
				want = types.StringValue("c-original")
			}
			if updated.Diagnostics.HasError() || result.ID.ValueString() != "w-existing" || !result.InitialClusterID.Equal(want) || requests != 3 {
				t.Fatalf("imported identity must round-trip through Read and Update: id=%s diagnostics=%v requests=%d", result.InitialClusterID, updated.Diagnostics, requests)
			}
		})
	}
}

func TestWarehouseReadUsesAuthoritativeIdentityWithoutGuessing(t *testing.T) {
	for _, field := range []string{`"initialClusterId":"c-original",`, `"initialClusterId":null,`, `"initialClusterId":"",`, ""} {
		for _, prior := range []types.String{types.StringNull(), types.StringUnknown(), types.StringValue("c-original"), types.StringValue("legacy-wrong")} {
			t.Run(field+prior.String(), func(t *testing.T) {
				reads := 0
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					reads++
					if req.Method != http.MethodGet || req.URL.Path != "/v1/warehouses/w-test" {
						t.Errorf("refresh must not list or mutate physical clusters: %s %s", req.Method, req.URL.Path)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					_, _ = w.Write([]byte(`{"success":true,"data":{` + field + `"warehouseId":"w-test"}}`))
				}))
				defer ts.Close()
				r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
				state := WarehouseResourceModel{InitialClusterID: prior}
				want := types.StringNull()
				if strings.Contains(field, "c-original") {
					want = types.StringValue("c-original")
				}
				for range 3 {
					var diagnostics diag.Diagnostics
					r.readWarehouseIntoState(context.Background(), "w-test", &state, &diagnostics)
					if diagnostics.HasError() || !state.InitialClusterID.Equal(want) {
						t.Fatalf("refresh must follow known/null/omitted API semantics: id=%s want=%s diagnostics=%v", state.InitialClusterID, want, diagnostics)
					}
				}
				if reads != 3 {
					t.Fatalf("expected warehouse reads only, got %d requests", reads)
				}
			})
		}
	}
}
