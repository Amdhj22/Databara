package analyzer

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Amdhj22/databara/internal/claude"
	"github.com/Amdhj22/databara/internal/strava"
)

// fakeCommenter records the request analyzer sent and returns a canned
// response. Used to verify Analyze wiring without hitting the real Claude
// API.
type fakeCommenter struct {
	seenReq claude.CommentRequest
	note    string
	err     error
}

func (f *fakeCommenter) Comment(_ context.Context, req claude.CommentRequest) (string, error) {
	f.seenReq = req
	return f.note, f.err
}

func TestAnalyze_PassesSportAndSummaryToClaude(t *testing.T) {
	fake := &fakeCommenter{note: "좋은 라이딩이었어요!"}
	a := New(fake)

	activity := strava.ActivityDetail{
		Activity: strava.Activity{
			ID:         99,
			Name:       "Hill repeats",
			SportType:  strava.SportRide,
			Distance:   20000,
			MovingTime: 3600,
		},
	}

	note, err := a.Analyze(context.Background(), activity)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}
	if note != "좋은 라이딩이었어요!" {
		t.Errorf("note = %q, want '좋은 라이딩이었어요!'", note)
	}
	if fake.seenReq.Sport != "Ride" {
		t.Errorf("Sport = %q, want Ride", fake.seenReq.Sport)
	}
	for _, want := range []string{"Name: Hill repeats", "Distance: 20.00 km", "Moving time: 1:00:00"} {
		if !strings.Contains(fake.seenReq.Summary, want) {
			t.Errorf("Summary missing %q; got:\n%s", want, fake.seenReq.Summary)
		}
	}
}

func TestAnalyze_PropagatesClaudeError(t *testing.T) {
	boom := errors.New("api down")
	fake := &fakeCommenter{err: boom}
	a := New(fake)

	_, err := a.Analyze(context.Background(), strava.ActivityDetail{
		Activity: strava.Activity{ID: 7, Name: "x"},
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want errors.Is %v", err, boom)
	}
	if !strings.Contains(err.Error(), "7") {
		t.Errorf("error should mention activity ID 7; got %v", err)
	}
}
