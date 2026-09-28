package resource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestReadManagedPublicAccessPolicyFailurePreservesState(t *testing.T) {
	for _, body := range []string{`{"success":true,"data":{}}`, `invalid json`} {
		t.Run(body, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, err := w.Write([]byte(body)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer ts.Close()
			r := &WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "test", 0, time.Second)}
			original := warehouseAccessPolicyListForTest("ALLOWLIST_ONLY", "203.0.113.0/24")
			state := WarehouseResourceModel{ID: types.StringValue("warehouse"), AccessPolicy: original}
			var diags diag.Diagnostics
			r.readManagedPublicAccessPolicy(context.Background(), &state, &diags)
			if !diags.HasError() {
				t.Fatal("expected read error")
			}
			if !state.AccessPolicy.Equal(original) {
				t.Fatal("failed read changed policy state")
			}
		})
	}
}

func TestReadUnmanagedPublicAccessPolicySkipsAPI(t *testing.T) {
	// A nil client would panic if an omitted/imported block attempted a read.
	state := WarehouseResourceModel{AccessPolicy: types.ListNull(warehouseAccessPolicyListForTest("DENY_ALL").ElementType(context.Background()))}
	var diags diag.Diagnostics
	(&WarehouseResource{}).readManagedPublicAccessPolicy(context.Background(), &state, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
}
