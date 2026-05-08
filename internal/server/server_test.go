package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Amdhj22/databara/internal/strava"
)

func newTestServer(t *testing.T, verifyToken string, handle func(context.Context, strava.WebhookEvent)) (*Server, *Worker, func()) {
	t.Helper()
	w := NewWorker(8, handle)
	ctx, cancel := context.WithCancel(context.Background())
	go w.Run(ctx)

	srv := New(verifyToken, w)
	cleanup := func() {
		cancel()
		w.Wait()
	}
	return srv, w, cleanup
}

func mountAndServe(srv *Server, req *http.Request) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	srv.Routes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestServer_Healthz(t *testing.T) {
	srv, _, cleanup := newTestServer(t, "secret", func(context.Context, strava.WebhookEvent) {})
	defer cleanup()

	rec := mountAndServe(srv, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want ok", rec.Body.String())
	}
}

func TestServer_StravaChallenge_HappyPath(t *testing.T) {
	srv, _, cleanup := newTestServer(t, "secret-token", func(context.Context, strava.WebhookEvent) {})
	defer cleanup()

	url := "/webhooks/strava?hub.mode=subscribe&hub.verify_token=secret-token&hub.challenge=abc"
	rec := mountAndServe(srv, httptest.NewRequest(http.MethodGet, url, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["hub.challenge"] != "abc" {
		t.Errorf("challenge echo = %q, want abc", body["hub.challenge"])
	}
}

func TestServer_StravaChallenge_RejectsWrongToken(t *testing.T) {
	srv, _, cleanup := newTestServer(t, "right", func(context.Context, strava.WebhookEvent) {})
	defer cleanup()

	url := "/webhooks/strava?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=abc"
	rec := mountAndServe(srv, httptest.NewRequest(http.MethodGet, url, nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestServer_StravaEvent_EnqueuesAndReturns200(t *testing.T) {
	var (
		mu       sync.Mutex
		received []int64
	)
	srv, _, cleanup := newTestServer(t, "secret", func(_ context.Context, ev strava.WebhookEvent) {
		mu.Lock()
		received = append(received, ev.ObjectID)
		mu.Unlock()
	})
	defer cleanup()

	body := `{
		"object_type":"activity",
		"object_id":12345,
		"aspect_type":"create",
		"owner_id":99,
		"subscription_id":7,
		"event_time":1700000000
	}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/strava", strings.NewReader(body))
	rec := mountAndServe(srv, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(received)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 || received[0] != 12345 {
		t.Errorf("worker received = %v, want [12345]", received)
	}
}

func TestServer_StravaEvent_RejectsBadBody(t *testing.T) {
	srv, _, cleanup := newTestServer(t, "secret", func(context.Context, strava.WebhookEvent) {
		t.Error("worker should not be called for malformed body")
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/webhooks/strava", strings.NewReader(`not json`))
	rec := mountAndServe(srv, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestServer_StravaEvent_FullQueueStillReturns200(t *testing.T) {
	// Build a worker with no consumer running and buf=1 so the second event
	// triggers ErrQueueFull. The handler must still respond 200 to keep
	// Strava from retrying into a deeper backlog.
	w := NewWorker(1, func(context.Context, strava.WebhookEvent) {})
	srv := New("secret", w)
	mux := http.NewServeMux()
	srv.Routes(mux)

	body := func(id int64) string {
		return strings.NewReplacer("__ID__",
			strconv.Itoa(int(id))).Replace(`{
				"object_type":"activity",
				"object_id":__ID__,
				"aspect_type":"create",
				"owner_id":99,
				"subscription_id":7,
				"event_time":1700000000
			}`)
	}
	for _, id := range []int64{1, 2} {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/strava", strings.NewReader(body(id)))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("event %d status = %d, want 200", id, rec.Code)
		}
	}
}
