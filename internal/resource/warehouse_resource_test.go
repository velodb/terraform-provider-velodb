package resource

import (
	"errors"
	"testing"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

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
