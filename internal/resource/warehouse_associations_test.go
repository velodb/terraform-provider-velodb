package resource

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestWarehouseReadAssociationIDs(t *testing.T) {
	for _, tc := range []struct {
		name, fields               string
		prior, credential, network types.Int64
	}{
		{"import", `,"credentialId":123,"networkConfigId":456`, types.Int64Null(), types.Int64Value(123), types.Int64Value(456)},
		{"refresh drift", `,"credentialId":123,"networkConfigId":456`, types.Int64Value(9), types.Int64Value(123), types.Int64Value(456)},
		{"old backend retains bindings", "", types.Int64Value(9), types.Int64Value(9), types.Int64Value(9)},
		{"old backend import", "", types.Int64Null(), types.Int64Null(), types.Int64Null()},
		{"old backend resolves computed unknown", "", types.Int64Unknown(), types.Int64Null(), types.Int64Null()},
		{"partial response", `,"networkConfigId":456`, types.Int64Value(9), types.Int64Value(9), types.Int64Value(456)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/warehouses/WH-TEST" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					http.NotFound(w, req)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if _, err := fmt.Fprintf(w, `{"success":true,"data":{"warehouseId":"WH-TEST"%s}}`, tc.fields); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			r := WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "test", 0, time.Second)}
			state := WarehouseResourceModel{CredentialID: tc.prior, NetworkConfigID: tc.prior, InitialClusterID: types.StringValue("cluster")}
			var diagnostics diag.Diagnostics
			r.readWarehouseIntoState(context.Background(), "WH-TEST", &state, &diagnostics)
			if diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			if !state.CredentialID.Equal(tc.credential) || !state.NetworkConfigID.Equal(tc.network) {
				t.Fatalf("got credential=%s network=%s; want credential=%s network=%s", state.CredentialID, state.NetworkConfigID, tc.credential, tc.network)
			}
		})
	}
}
