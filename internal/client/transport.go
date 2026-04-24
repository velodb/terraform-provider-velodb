package client

import (
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// formationTransport is a custom http.RoundTripper that handles:
// - X-API-Key authentication header injection
// - RequestId idempotency header for write operations
// - Retry with exponential backoff for 429 and 503 responses
type formationTransport struct {
	base       http.RoundTripper
	apiKey     string
	maxRetries int
}

func (t *formationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Inject auth header
	req.Header.Set("X-API-Key", t.apiKey)
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// For write operations, generate RequestId if not already set
	if isWriteMethod(req.Method) && req.Header.Get("RequestId") == "" {
		req.Header.Set("RequestId", uuid.NewString())
	}

	// Execute with retry + backoff
	var resp *http.Response
	var err error
	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		if attempt > 0 {
			wait := backoffDuration(attempt, resp)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(wait):
			}
		}

		resp, err = t.base.RoundTrip(req)
		if err != nil {
			continue // network error, retry
		}
		if !isRetryableStatus(resp.StatusCode) {
			break
		}
	}
	return resp, err
}

func isWriteMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodPatch || method == http.MethodDelete || method == http.MethodPut
}

func isRetryableStatus(status int) bool {
	return status == 429 || status == 503
}

func backoffDuration(attempt int, resp *http.Response) time.Duration {
	// Respect Retry-After header if present
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				return time.Duration(secs) * time.Second
			}
		}
	}

	// Exponential backoff: 1s, 2s, 4s, 8s... capped at 30s, with jitter
	base := math.Pow(2, float64(attempt-1))
	if base > 30 {
		base = 30
	}
	jitter := rand.Float64() // 0-1s jitter
	return time.Duration((base+jitter)*1000) * time.Millisecond
}
