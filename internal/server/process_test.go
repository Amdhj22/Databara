package server

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Amdhj22/databara/internal/strava"
)

// fakeFetcher records the activity ID requested and returns a canned reply.
type fakeFetcher struct {
	mu       sync.Mutex
	seenID   int64
	activity strava.ActivityDetail
	err      error
}

func (f *fakeFetcher) GetActivity(_ context.Context, id int64) (strava.ActivityDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seenID = id
	return f.activity, f.err
}

type fakeReviewer struct {
	mu          sync.Mutex
	seenSportID int64
	sport       strava.SportType
	note        string
	err         error
}

func (f *fakeReviewer) Analyze(_ context.Context, a strava.ActivityDetail) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seenSportID = a.ID
	f.sport = a.SportType
	return f.note, f.err
}

type fakeSender struct {
	mu       sync.Mutex
	seenChat int64
	seenText string
	err      error
}

func (f *fakeSender) SendMessage(_ context.Context, chatID int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seenChat = chatID
	f.seenText = text
	return f.err
}

func newProcessor(f *fakeFetcher, r *fakeReviewer, s *fakeSender, chatID int64) *Processor {
	return &Processor{Fetcher: f, Reviewer: r, Sender: s, ChatID: chatID}
}

func createEvent(id int64) strava.WebhookEvent {
	return strava.WebhookEvent{
		ObjectType: strava.ObjectTypeActivity,
		ObjectID:   id,
		AspectType: strava.AspectCreate,
		OwnerID:    99,
	}
}

func TestProcessor_Handle_HappyPath(t *testing.T) {
	fetcher := &fakeFetcher{
		activity: strava.ActivityDetail{
			Activity: strava.Activity{ID: 42, SportType: strava.SportRide, Name: "x"},
		},
	}
	reviewer := &fakeReviewer{note: "좋은 라이딩이었어요!"}
	sender := &fakeSender{}
	p := newProcessor(fetcher, reviewer, sender, 12345)

	p.Handle(context.Background(), createEvent(42))

	if fetcher.seenID != 42 {
		t.Errorf("fetcher saw id=%d, want 42", fetcher.seenID)
	}
	if reviewer.seenSportID != 42 || reviewer.sport != strava.SportRide {
		t.Errorf("reviewer saw id=%d sport=%s, want 42/Ride", reviewer.seenSportID, reviewer.sport)
	}
	if sender.seenChat != 12345 || sender.seenText != "좋은 라이딩이었어요!" {
		t.Errorf("sender got chat=%d text=%q", sender.seenChat, sender.seenText)
	}
}

func TestProcessor_Handle_SkipsNonCreateEvents(t *testing.T) {
	fetcher := &fakeFetcher{}
	reviewer := &fakeReviewer{}
	sender := &fakeSender{}
	p := newProcessor(fetcher, reviewer, sender, 1)

	p.Handle(context.Background(), strava.WebhookEvent{
		ObjectType: strava.ObjectTypeActivity,
		ObjectID:   42,
		AspectType: strava.AspectUpdate,
	})

	if fetcher.seenID != 0 {
		t.Errorf("fetcher should not have been called for update event; saw id=%d", fetcher.seenID)
	}
	if sender.seenChat != 0 {
		t.Error("sender should not have been called for update event")
	}
}

func TestProcessor_Handle_StopsOnFetchError(t *testing.T) {
	fetcher := &fakeFetcher{err: errors.New("strava down")}
	reviewer := &fakeReviewer{}
	sender := &fakeSender{}
	p := newProcessor(fetcher, reviewer, sender, 1)

	p.Handle(context.Background(), createEvent(42))

	if reviewer.seenSportID != 0 {
		t.Error("reviewer should not run when fetch fails")
	}
	if sender.seenChat != 0 {
		t.Error("sender should not run when fetch fails")
	}
}

func TestProcessor_Handle_StopsOnReviewError(t *testing.T) {
	fetcher := &fakeFetcher{
		activity: strava.ActivityDetail{Activity: strava.Activity{ID: 42}},
	}
	reviewer := &fakeReviewer{err: errors.New("claude down")}
	sender := &fakeSender{}
	p := newProcessor(fetcher, reviewer, sender, 1)

	p.Handle(context.Background(), createEvent(42))

	if sender.seenChat != 0 {
		t.Error("sender should not run when review fails")
	}
}

func TestSenderFunc_AdaptsClosure(t *testing.T) {
	var gotChat int64
	var gotText string
	f := SenderFunc(func(_ context.Context, chatID int64, text string) error {
		gotChat = chatID
		gotText = text
		return nil
	})

	if err := f.SendMessage(context.Background(), 7, "hi"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if gotChat != 7 || gotText != "hi" {
		t.Errorf("got chat=%d text=%q", gotChat, gotText)
	}
}
