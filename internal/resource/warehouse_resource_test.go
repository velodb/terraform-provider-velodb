package resource

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

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
