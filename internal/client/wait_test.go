package client

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWaitForStatusDoesNotTreatEmptyStatusAsDeleted(t *testing.T) {
	_, err := WaitForStatus(context.Background(), func(context.Context) (string, error) {
		return "", nil
	}, []string{"Deleted"}, FailedStatuses, 20*time.Millisecond, time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout")
	}
}

func TestWaitForStatusDoesNotTreatEmptyStatusAsStable(t *testing.T) {
	_, err := WaitForStatus(context.Background(), func(context.Context) (string, error) {
		return "", nil
	}, []string{"Running"}, FailedStatuses, 20*time.Millisecond, time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout")
	}
	if !strings.Contains(err.Error(), "timed out waiting for status") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestWaitForStatusRetriesTransientPollingErrors(t *testing.T) {
	calls := 0
	status, err := WaitForStatus(context.Background(), func(context.Context) (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("temporary dns failure")
		}
		return "Running", nil
	}, []string{"Running"}, FailedStatuses, time.Second, time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForStatus: %v", err)
	}
	if status != "Running" {
		t.Fatalf("expected Running, got %q", status)
	}
	if calls != 2 {
		t.Fatalf("expected two polls, got %d", calls)
	}
}
