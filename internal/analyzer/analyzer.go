package analyzer

import (
	"context"
	"fmt"

	"github.com/Amdhj22/databara/internal/claude"
	"github.com/Amdhj22/databara/internal/strava"
)

// Commenter is the slice of claude.Client this package needs. Defining it
// locally lets tests swap in a fake without standing up an HTTP server, and
// keeps the import boundary one-way (analyzer → claude, never the reverse).
type Commenter interface {
	Comment(ctx context.Context, req claude.CommentRequest) (string, error)
}

// Analyzer composes the formatter with the language model: format the
// activity into a Summary, then ask Claude for a coaching note.
type Analyzer struct {
	Claude Commenter
}

// New returns an Analyzer that delegates the LLM call to c.
func New(c Commenter) *Analyzer {
	return &Analyzer{Claude: c}
}

// Analyze formats the activity, asks Claude for a coaching note, and returns
// the note ready for downstream pushing (e.g. Telegram). Errors from the
// formatter are not possible — Claude errors are wrapped with the activity
// ID so the log line is self-describing.
func (a *Analyzer) Analyze(ctx context.Context, activity strava.ActivityDetail) (string, error) {
	req := claude.CommentRequest{
		Sport:   string(activity.SportType),
		Summary: formatActivity(activity),
	}
	note, err := a.Claude.Comment(ctx, req)
	if err != nil {
		return "", fmt.Errorf("analyze activity %d: %w", activity.ID, err)
	}
	return note, nil
}
