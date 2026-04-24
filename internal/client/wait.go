package client

import (
	"context"
	"fmt"
	"time"
)

// StatusFunc is a function that returns the current status of a resource.
type StatusFunc func(ctx context.Context) (string, error)

// WaitForStatus polls until the resource reaches one of the target statuses,
// or returns an error if it reaches a failed status or times out.
func WaitForStatus(ctx context.Context, fetchStatus StatusFunc, targetStatuses, failedStatuses []string, timeout, interval time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	targetSet := toSet(targetStatuses)
	failedSet := toSet(failedStatuses)

	for {
		status, err := fetchStatus(ctx)
		if err != nil {
			// If we get a 404 and "Deleted" is a target, treat that as success
			if apiErr, ok := err.(*APIError); ok && apiErr.IsNotFound() {
				if _, ok := targetSet["Deleted"]; ok {
					return "Deleted", nil
				}
			}
			return "", fmt.Errorf("polling status: %w", err)
		}

		if _, ok := targetSet[status]; ok {
			return status, nil
		}
		if _, ok := failedSet[status]; ok {
			return status, fmt.Errorf("resource reached failed status: %s", status)
		}

		select {
		case <-ctx.Done():
			return status, fmt.Errorf("timed out waiting for status %v, current status: %s", targetStatuses, status)
		case <-ticker.C:
		}
	}
}

func toSet(ss []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}

// Common status classifications.
var (
	StableStatuses = []string{"Running", "Stopped", "Suspended"}
	FailedStatuses = []string{"CreateFailed"}
)
