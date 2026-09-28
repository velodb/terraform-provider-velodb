package resource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestCoreVersionSelection(t *testing.T) {
	for _, tc := range []struct {
		target, current string
		reject          bool
	}{
		{"4.1.9", "4.1.5", false}, {"4.1.5", "4.1.9", true},
		{"4.1", "4.1.5", false}, {"4.2", "4.1.5", true},
		{"4.10.0", "4.9.9", false}, {"4.9.9", "4.10.0", true},
	} {
		t.Run(tc.target+" from "+tc.current, func(t *testing.T) {
			if err := validateCoreVersionUpgrade(tc.target, tc.current); (err != nil) != tc.reject {
				t.Fatalf("err=%v", err)
			}
		})
	}
	for _, tc := range []struct {
		target       types.String
		actual, want string
	}{
		{types.StringValue("4.1"), "4.1.5", "4.1"},
		{types.StringValue("4.1.5"), "4.1.9", "4.1.9"},
		{types.StringNull(), "4.1.5", "4.1.5"},
		{types.StringValue("4.1"), "4.2.1", "4.2.1"},
	} {
		if got := coreVersionForState(tc.target, tc.actual).ValueString(); got != tc.want {
			t.Fatalf("got %s want %s", got, tc.want)
		}
	}
}

func TestResolveCoreVersionID(t *testing.T) {
	for _, tc := range []struct {
		name     string
		versions []client.WarehouseVersion
		want     int64
		reject   bool
	}{
		{"exact", []client.WarehouseVersion{{Version: "4.1.5", VersionID: 5}, {Version: "4.1.9", VersionID: 9}}, 9, false},
		{"no target", []client.WarehouseVersion{{Version: "4.1.5", VersionID: 5}}, 0, true},
		{"ambiguous", []client.WarehouseVersion{{Version: "4.1.9", VersionID: 9}, {Version: "4.1.9", VersionID: 19}}, 0, true},
		{"invalid id", []client.WarehouseVersion{{Version: "4.1.9", VersionID: 0}}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveCoreVersionID("4.1.9", tc.versions)
			if got != tc.want || (err != nil) != tc.reject {
				t.Fatalf("id=%d err=%v", got, err)
			}
		})
	}
}

func TestCoreVersionUpgradeReadsActualPatch(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		wantID       int64
		wantError    bool
		wantLists    int
	}{
		{"same actual version", "4.1.9", 0, false, 0},
		{"reject hidden downgrade", "4.1.5", 0, true, 0},
		{"upgrade exact patch", "4.1.11", 11, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads, lists := 0, 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body := `{"success":true,"data":{"warehouseId":"WH","coreVersion":"4.1.9"}}`
				if strings.HasSuffix(r.URL.Path, "/versions") {
					lists++
					body = `{"success":true,"data":[{"version":"4.1.11","versionId":11}]}`
				} else {
					reads++
				}
				if _, err := w.Write([]byte(body)); err != nil {
					t.Errorf("write: %v", err)
				}
			}))
			defer ts.Close()
			r := WarehouseResource{client: client.NewFormationClient(strings.TrimPrefix(ts.URL, "http://"), "test", 0, time.Second)}
			state := WarehouseResourceModel{ID: types.StringValue("WH"), CoreVersion: types.StringValue("4.1")}
			id, err := r.coreVersionUpgradeID(context.Background(), types.StringValue(tc.target), &WarehouseResourceModel{}, &state)
			if id != tc.wantID || (err != nil) != tc.wantError || reads != 1 || lists != tc.wantLists {
				t.Fatalf("id=%d err=%v reads=%d lists=%d", id, err, reads, lists)
			}
		})
	}
}
