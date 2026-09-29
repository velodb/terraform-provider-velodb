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

func TestWarehouseReadTableNameCaseSensitivity(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		prior, want types.Bool
		wantError   bool
	}{
		{"case-sensitive import", `,"lowerCaseMode":0`, types.BoolNull(), types.BoolValue(true), false},
		{"case-insensitive import", `,"lowerCaseMode":1`, types.BoolNull(), types.BoolValue(false), false},
		{"refresh drift", `,"lowerCaseMode":1`, types.BoolValue(true), types.BoolValue(false), false},
		{"legacy backend retains state", "", types.BoolValue(false), types.BoolValue(false), false},
		{"legacy backend import stays null", "", types.BoolNull(), types.BoolNull(), false},
		{"invalid API value", `,"lowerCaseMode":2`, types.BoolNull(), types.BoolNull(), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if _, err := fmt.Fprintf(w, `{"success":true,"data":{"warehouseId":"WH-TEST"%s}}`, tc.field); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()

			r := WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(server.URL, "http://"), "test", 0, time.Second)}
			state := WarehouseResourceModel{TableNameCaseSensitive: tc.prior, InitialClusterID: types.StringValue("cluster")}
			var diagnostics diag.Diagnostics
			r.readWarehouseIntoState(context.Background(), "WH-TEST", &state, &diagnostics)
			if diagnostics.HasError() != tc.wantError {
				t.Fatalf("diagnostics = %v, wantError = %v", diagnostics, tc.wantError)
			}
			if !state.TableNameCaseSensitive.Equal(tc.want) {
				t.Fatalf("TableNameCaseSensitive = %s, want %s", state.TableNameCaseSensitive, tc.want)
			}
		})
	}
}
