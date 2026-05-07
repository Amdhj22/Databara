package strava

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerifyChallenge_HappyPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet,
		"/?hub.mode=subscribe&hub.verify_token=secret&hub.challenge=abc123", nil)
	rec := httptest.NewRecorder()

	VerifyChallenge(rec, req, "secret")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["hub.challenge"] != "abc123" {
		t.Errorf("hub.challenge = %q, want abc123", body["hub.challenge"])
	}
}

func TestVerifyChallenge_RejectsCases(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantCode int
	}{
		{"wrong mode", "/?hub.mode=unsubscribe&hub.verify_token=secret&hub.challenge=x", http.StatusBadRequest},
		{"bad token", "/?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=x", http.StatusForbidden},
		{"missing challenge", "/?hub.mode=subscribe&hub.verify_token=secret", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()
			VerifyChallenge(rec, req, "secret")
			if rec.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantCode)
			}
		})
	}
}

func TestDecodeWebhookEvent(t *testing.T) {
	payload := `{
		"object_type":"activity",
		"object_id":12345,
		"aspect_type":"create",
		"owner_id":99,
		"subscription_id":7,
		"event_time":1700000000
	}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))

	ev, err := DecodeWebhookEvent(req)
	mustNoError(t, err)

	if !ev.IsActivityCreate() {
		t.Errorf("IsActivityCreate = false, want true")
	}
	if ev.ObjectID != 12345 || ev.OwnerID != 99 {
		t.Errorf("decoded = %+v, want object 12345 / owner 99", ev)
	}
}

func TestDecodeWebhookEvent_RejectsMissingFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"object_id":1}`))
	if _, err := DecodeWebhookEvent(req); err == nil {
		t.Fatal("expected error for missing object_type/aspect_type")
	}
}

func TestRegisterSubscription_PostsForm(t *testing.T) {
	// We cannot redirect subscriptionsURL from the outside without mutating
	// the constant, so this test exercises the form-building path by hitting
	// a stub at the well-known URL via httptest.NewUnstartedServer/Listener
	// override is overkill here. Instead, verify that doSubscriptionRequest
	// drives an arbitrary request through HTTPClient and decodes the body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		_, _ = w.Write([]byte(`{"id":42,"application_id":1,"callback_url":"https://x"}`))
	}))
	defer srv.Close()

	c := New("cid", "csec", newMemTokens(validToken()))
	c.HTTPClient = srv.Client()

	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader("k=v"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var got Subscription
	mustNoError(t, c.doSubscriptionRequest(req, &got))

	if got.ID != 42 {
		t.Errorf("Subscription.ID = %d, want 42", got.ID)
	}
}
