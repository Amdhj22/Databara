package server

import (
	"context"
	"errors"

	"github.com/Amdhj22/databara/internal/strava"
)

// ErrQueueFull is returned by Worker.Enqueue when the buffered channel has
// no room. The caller (the HTTP handler) decides what to do — Phase 1 logs
// and returns 200 anyway so Strava doesn't retry into a deeper backlog.
var ErrQueueFull = errors.New("server: worker queue full")

// Worker is a single-consumer pump that funnels webhook events into a
// handler function. The buffered channel decouples the HTTP layer (must
// respond fast to Strava) from the slow processing pipeline (Strava fetch +
// Claude call + Telegram push, all network-bound).
//
// One consumer goroutine is enough for Phase 1 because activities arrive
// one at a time per athlete. Add a small worker pool if event fan-out
// grows.
type Worker struct {
	queue  chan strava.WebhookEvent
	handle func(context.Context, strava.WebhookEvent)
	done   chan struct{}
}

// NewWorker returns a Worker with a buffered queue of the given size. The
// handle function is called once per dequeued event, on the goroutine that
// runs Worker.Run. handle should not block longer than a single end-to-end
// processing pass; long blocks back-pressure the HTTP layer.
func NewWorker(buf int, handle func(context.Context, strava.WebhookEvent)) *Worker {
	return &Worker{
		queue:  make(chan strava.WebhookEvent, buf),
		handle: handle,
		done:   make(chan struct{}),
	}
}

// Enqueue tries to submit ev for processing without blocking. It returns
// ErrQueueFull if the queue is at capacity so the HTTP handler can decide
// whether to drop or stall.
func (w *Worker) Enqueue(ev strava.WebhookEvent) error {
	select {
	case w.queue <- ev:
		return nil
	default:
		return ErrQueueFull
	}
}

// Run blocks on the queue, dispatching every event to handle until ctx is
// cancelled. It is intended to run in a dedicated goroutine; main.go waits
// on Wait() during shutdown.
//
// Events still in the queue when ctx cancels are dropped — Phase 1 chose
// not to drain on shutdown because the pipeline calls external APIs that
// would themselves block on a cancelled context.
func (w *Worker) Run(ctx context.Context) {
	defer close(w.done)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-w.queue:
			w.handle(ctx, ev)
		}
	}
}

// Wait blocks until Run returns. Call this after the parent context has
// been cancelled to give the in-flight handler a chance to finish.
func (w *Worker) Wait() {
	<-w.done
}
