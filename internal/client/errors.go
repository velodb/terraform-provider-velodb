package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// APIError represents a non-success API response.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d [%s]: %s (requestId=%s)", e.StatusCode, e.Code, e.Message, e.RequestID)
}

// UserMessage returns a user-friendly message with recovery guidance.
func (e *APIError) UserMessage() string {
	base := e.Error()
	switch {
	case e.StatusCode == 401:
		return base + "\n\nAuthentication failed — verify your api_key is correct and not expired."
	case e.StatusCode == 403:
		return base + "\n\nPermission denied — check that your API key has the required permissions."
	case e.StatusCode == 404:
		return base + "\n\nResource not found — verify the resource ID exists and has not been deleted."
	case e.StatusCode == 409:
		return base + "\n\nConflict — the resource is in a state that does not allow this operation. " +
			"Wait for the current operation to complete and re-run terraform apply."
	case e.StatusCode == 429:
		return base + "\n\nRate limited — too many API requests. Wait a moment and re-run terraform apply."
	case e.Code == "WarehouseNotFound" || e.Code == "ClusterNotFound":
		return base + "\n\nThe resource no longer exists. Run terraform plan to refresh state."
	default:
		return base + "\n\nFix the configuration and re-run terraform apply."
	}
}

// IsNotFound returns true if the error indicates the resource was not found.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404 ||
		e.Code == "WarehouseNotFound" ||
		e.Code == "ClusterNotFound"
}

// IsConflict returns true if the error indicates a conflict (e.g., idempotency).
func (e *APIError) IsConflict() bool {
	return e.StatusCode == 409
}

// IsRateLimited returns true if the error indicates rate limiting.
func (e *APIError) IsRateLimited() bool {
	return e.StatusCode == 429
}

// parseResponse reads an HTTP response and decodes the JSON body into result.
// It returns an APIError for non-2xx responses.
func parseResponse[T any](resp *http.Response, result *T) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"requestId"`
		}
		_ = json.Unmarshal(body, &errResp)
		return &APIError{
			StatusCode: resp.StatusCode,
			Code:       errResp.Code,
			Message:    errResp.Message,
			RequestID:  errResp.RequestID,
		}
	}

	// 202 Accepted — idempotent request still in flight, not an error but callers should handle
	if resp.StatusCode == 202 && result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("decoding 202 response: %w", err)
		}
		return nil
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}
