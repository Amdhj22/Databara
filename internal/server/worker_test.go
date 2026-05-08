package server

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Amdhj22/databara/internal/strava"
)

func TestWorker_DispatchesEnqueuedEvents(t *testing.T) {
	var (
		mu       sync.Mutex
		received []int64
	)
	w := NewWorker(4, func(_ context.Context, ev strava.WebhookEvent) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, ev.ObjectID)
	})

	ctx, cancel := context.WithCancel(context.Background())
	go w.Run(ctx)

	for _, id := range []int64{1, 2, 3} {
		if err := w.Enqueue(strava.WebhookEvent{ObjectID: id}); err != nil {
			t.Fatalf("Enqueue %d: %v", id, err)
		}
	}

	// Spin briefly until all three events are observed. 200 ms is generous —
	// each handler call here is in-process and microseconds long.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(received)
		mu.Unlock()
		if n == 3 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	cancel()
	w.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 3 {
		t.Fatalf("received = %v, want length 3", received)
	}
	for i, want := range []int64{1, 2, 3} {
		if received[i] != want {
			t.Errorf("received[%d] = %d, want %d", i, received[i], want)
		}
	}
}

func TestWorker_EnqueueReturnsErrQueueFull(t *testing.T) {
	// Build a Worker without starting Run, so the queue can't drain. Buf=2
	// means the third Enqueue must fail.
	w := NewWorker(2, func(context.Context, strava.WebhookEvent) {})

	if err := w.Enqueue(strava.WebhookEvent{ObjectID: 1}); err != nil {
		t.Fatalf("first Enqueue: %v", err)
	}
	if err := w.Enqueue(strava.WebhookEvent{ObjectID: 2}); err != nil {
		t.Fatalf("second Enqueue: %v", err)
	}
	if err := w.Enqueue(strava.WebhookEvent{ObjectID: 3}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("third Enqueue err = %v, want ErrQueueFull", err)
	}
}

func TestWorker_RunReturnsOnContextCancel(t *testing.T) {
	var calls atomic.Int64
	w := NewWorker(1, func(context.Context, strava.WebhookEvent) {
		calls.Add(1)
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("handler ran %d times despite no events queued", got)
	}
}
