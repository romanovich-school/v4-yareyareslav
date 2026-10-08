package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitWorkerFinished(t *testing.T) {
	done := make(chan struct{})
	close(done)
	if err := waitWorker(context.Background(), done); err != nil {
		t.Fatal(err)
	}
}

func TestWaitWorkerTimesOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := waitWorker(ctx, make(chan struct{})); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want DeadlineExceeded", err)
	}
}
