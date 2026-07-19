package search

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithTimeoutReturnsFnResult(t *testing.T) {
	got, err := withTimeout(context.Background(), time.Second, func() (string, error) {
		return "ok", nil
	})
	if err != nil || got != "ok" {
		t.Fatalf("got (%q, %v), want (\"ok\", nil)", got, err)
	}
}

func TestWithTimeoutPropagatesFnError(t *testing.T) {
	wantErr := errors.New("boom")
	_, err := withTimeout(context.Background(), time.Second, func() (string, error) {
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("got err %v, want %v", err, wantErr)
	}
}

func TestWithTimeoutGivesUpOnStalledFn(t *testing.T) {
	start := time.Now()
	_, err := withTimeout(context.Background(), 20*time.Millisecond, func() (string, error) {
		select {} // simulates a blocking syscall that never returns
	})
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("took %s to give up, want close to the 20ms timeout", elapsed)
	}
}

func TestWithTimeoutRespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := withTimeout(ctx, 5*time.Second, func() (string, error) {
			select {} // never returns on its own
		})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("got err %v, want context.Canceled", err)
		}
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("withTimeout did not return promptly after ctx cancellation")
	}
}
