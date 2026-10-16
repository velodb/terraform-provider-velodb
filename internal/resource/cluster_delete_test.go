package resource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestClusterDeleteConflictRetainsStateAndExplainsRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		status        int
	}{
		{"bound member", "cluster is bound to a virtual cluster", http.StatusConflict},
		{"other conflict", "another operation is running", http.StatusConflict},
		{"denied", "permission denied", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls++
				if req.Method != http.MethodDelete || req.URL.Path != "/v1/warehouses/w-test/clusters/c-old" {
					t.Errorf("unexpected mutation or poll: %s %s", req.Method, req.URL.Path)
				}
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"code": "OperationConflict", "message": tc.message, "requestId": "request-test"})
			}))
			defer ts.Close()
			ctx := context.Background()
			r := &ClusterResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "key", 0, time.Second)}
			var schema resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schema)
			ty := schema.Schema.Type().TerraformType(ctx).(tftypes.Object)
			values := map[string]tftypes.Value{}
			for name, attrType := range ty.AttributeTypes {
				values[name] = tftypes.NewValue(attrType, nil)
			}
			values["id"] = tftypes.NewValue(tftypes.String, "c-old")
			values["warehouse_id"] = tftypes.NewValue(tftypes.String, "w-test")
			state := tfsdk.State{Schema: schema.Schema, Raw: tftypes.NewValue(ty, values)}
			resp := resource.DeleteResponse{State: state}
			r.Delete(ctx, resource.DeleteRequest{State: state}, &resp)
			if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) || calls != 1 {
				t.Fatalf("failed delete must retain state without additional operations: %v, calls=%d", resp.Diagnostics, calls)
			}
			detail := resp.Diagnostics[0].Detail()
			if !strings.Contains(detail, tc.message) || !strings.Contains(detail, "request-test") {
				t.Fatalf("must preserve API cause and request ID: %s", detail)
			}
			for _, hint := range []string{"If this cluster is still bound", "keep or restore", "later apply", "create_before_destroy", "For other conflicts"} {
				if strings.Contains(detail, hint) != (tc.status == http.StatusConflict) {
					t.Fatalf("recovery hint %q must be conditional and specific to deletion conflicts: %s", hint, detail)
				}
			}
		})
	}
}
