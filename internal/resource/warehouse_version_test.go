package resource

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/velodb/terraform-provider-velodb/internal/client"
	"testing"
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
