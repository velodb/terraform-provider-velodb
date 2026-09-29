package resource

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

func TestWarehouseAssociationIDsAreOptionalComputed(t *testing.T) {
	var resp resource.SchemaResponse
	(&WarehouseResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)

	for _, name := range []string{"credential_id", "network_config_id", "tde_encryption_key_id", "ebs_encryption_key_id"} {
		attribute, ok := resp.Schema.Attributes[name].(schema.Int64Attribute)
		if !ok {
			t.Fatalf("%s is %T, want schema.Int64Attribute", name, resp.Schema.Attributes[name])
		}
		if !attribute.Optional || !attribute.Computed {
			t.Fatalf("%s must be optional and computed to support configuration and API readback", name)
		}
		if len(attribute.PlanModifiers) == 0 {
			t.Fatalf("%s must reject changes when the configured and API values differ", name)
		}
	}
}

func TestWarehouseTableNameCaseSensitivity(t *testing.T) {
	var resp resource.SchemaResponse
	(&WarehouseResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)

	attribute, ok := resp.Schema.Attributes["table_name_case_sensitive"].(schema.BoolAttribute)
	if !ok {
		t.Fatalf("table_name_case_sensitive is %T, want schema.BoolAttribute", resp.Schema.Attributes["table_name_case_sensitive"])
	}
	if !attribute.Optional || !attribute.Computed || len(attribute.PlanModifiers) == 0 {
		t.Fatal("table_name_case_sensitive must be optional, computed, and reject changes")
	}

	for _, tt := range []struct {
		name string
		in   types.Bool
		want *int
	}{
		{name: "omitted", in: types.BoolNull()},
		{name: "unknown", in: types.BoolUnknown()},
		{name: "case sensitive", in: types.BoolValue(true), want: intPointer(0)},
		{name: "case insensitive", in: types.BoolValue(false), want: intPointer(1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got *int
			setLowerCaseMode(&got, tt.in)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("setLowerCaseMode() = %d, want nil", *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("setLowerCaseMode() = %v, want %d", got, *tt.want)
			}
		})
	}
}

func intPointer(value int) *int { return &value }

func TestWarehouseDeleteInProgress(t *testing.T) {
	if !warehouseDeleteInProgress(&client.APIError{Code: "OperationConflict", Message: "warehouse is already in deleting status"}) {
		t.Fatal("expected an already-running deletion to be retryable")
	}
	if warehouseDeleteInProgress(&client.APIError{Code: "OperationConflict", Message: "warehouse is being upgraded"}) {
		t.Fatal("unexpected retry for an unrelated operation conflict")
	}
	if warehouseDeleteInProgress(errors.New("temporary failure")) {
		t.Fatal("unexpected retry for a non-API error")
	}
}
