package server

import (
	"context"
	"log/slog"

	"github.com/Amdhj22/databara/internal/strava"
)

// Fetcher loads the full ActivityDetail for a Strava activity ID. The strava
// package's Client satisfies this with no adapter.
type Fetcher interface {
	GetActivity(ctx context.Context, id int64) (strava.ActivityDetail, error)
}

// Reviewer turns an activity into a coaching note. The analyzer package's
// Analyzer satisfies this with no adapter.
type Reviewer interface {
	Analyze(ctx context.Context, activity strava.ActivityDetail) (string, error)
}

// Sender pushes the coaching note to the user. Kept variadic-free here so
// telegram.SendOption does not leak into the server package; main.go wires
// the real Telegram client through SenderFunc.
type Sender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// SenderFunc adapts an ordinary function into a Sender. Useful at the wiring
// boundary: telegram.Client.SendMessage takes variadic options, so main.go
// supplies a closure that forwards a fixed option set.
type SenderFunc func(ctx context.Context, chatID int64, text string) error

// SendMessage implements Sender for SenderFunc.
func (f SenderFunc) SendMessage(ctx context.Context, chatID int64, text string) error {
	return f(ctx, chatID, text)
}

// Processor drives the per-event pipeline: fetch the activity, ask the
// reviewer for a note, push it via the sender. ChatID is the destination
// for Phase 1 (single-user PoC).
//
// Errors are logged and dropped rather than propagated — the caller is the
// worker goroutine, which has no useful retry policy at this layer. Phase 2
// can add a dead-letter or backoff queue if reliability becomes a concern.
type Processor struct {
	Fetcher  Fetcher
	Reviewer Reviewer
	Sender   Sender
	ChatID   int64
}

// Handle processes one webhook event. Non-create events (edits, deletes)
// are skipped — Phase 1 only reacts to fresh uploads.
func (p *Processor) Handle(ctx context.Context, ev strava.WebhookEvent) {
	if !ev.IsActivityCreate() {
		slog.Debug("skip non-create webhook event",
			"object_type", ev.ObjectType, "aspect_type", ev.AspectType)
		return
	}

	activity, err := p.Fetcher.GetActivity(ctx, ev.ObjectID)
	if err != nil {
		slog.Error("fetch activity failed", "id", ev.ObjectID, "err", err)
		return
	}

	note, err := p.Reviewer.Analyze(ctx, activity)
	if err != nil {
		slog.Error("review failed", "id", ev.ObjectID, "err", err)
		return
	}

	if err := p.Sender.SendMessage(ctx, p.ChatID, note); err != nil {
		slog.Error("telegram send failed", "id", ev.ObjectID, "chat_id", p.ChatID, "err", err)
		return
	}

	slog.Info("activity processed",
		"id", ev.ObjectID, "sport", activity.SportType, "chat_id", p.ChatID)
}
