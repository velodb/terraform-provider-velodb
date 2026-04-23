package client

import (
	"context"
	"net/http"
	"testing"
)

func TestTransportInjectsAPIKey(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/warehouses", func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key != "test-api-key" {
			t.Errorf("expected X-API-Key 'test-api-key', got %q", key)
		}
		jsonResponse(w, 200, PageResponse[WarehouseItem]{
			Success:   true,
			RequestID: "req-t1",
			Data:      []WarehouseItem{},
			Page:      1,
			Size:      20,
			Total:     0,
		})
	})

	_, err := client.ListWarehouses(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListWarehouses: %v", err)
	}
}

func TestTransportGeneratesRequestIdForWrites(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	var capturedRequestID string
	mux.HandleFunc("/v1/warehouses/WH-001", func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = r.Header.Get("RequestId")
		jsonResponse(w, 200, APIResponse[struct{}]{Success: true, RequestID: "req-t2"})
	})

	client.DeleteWarehouse(context.Background(), "WH-001")
	if capturedRequestID == "" {
		t.Error("expected RequestId header to be auto-generated for DELETE")
	}
}

func TestTransportNoRequestIdForReads(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	var capturedRequestID string
	mux.HandleFunc("/v1/warehouses/WH-001", func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = r.Header.Get("RequestId")
		jsonResponse(w, 200, APIResponse[WarehouseItem]{
			Success:   true,
			RequestID: "req-t3",
			Data:      mockWarehouse("WH-001", "test"),
		})
	})

	client.GetWarehouse(context.Background(), "WH-001")
	if capturedRequestID != "" {
		t.Errorf("expected no RequestId for GET, got %q", capturedRequestID)
	}
}

func TestTransportRetryOn503(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()

	// Client with 1 retry
	host := ts.URL[len("http://"):]
	c := &FormationClient{
		BaseURL: ts.URL,
		HTTPClient: &http.Client{
			Transport: &formationTransport{
				base:       http.DefaultTransport,
				apiKey:     "test-api-key",
				maxRetries: 1,
			},
		},
	}
	_ = host

	attempt := 0
	mux.HandleFunc("/v1/warehouses/WH-001", func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			jsonResponse(w, 503, map[string]any{
				"code":    "ServiceUnavailable",
				"message": "Service is temporarily unavailable",
				"success": false,
			})
			return
		}
		jsonResponse(w, 200, APIResponse[WarehouseItem]{
			Success:   true,
			RequestID: "req-t4",
			Data:      mockWarehouse("WH-001", "test"),
		})
	})

	wh, err := c.GetWarehouse(context.Background(), "WH-001")
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if wh.Name != "test" {
		t.Errorf("expected name 'test', got %q", wh.Name)
	}
	if attempt != 2 {
		t.Errorf("expected 2 attempts, got %d", attempt)
	}
}
