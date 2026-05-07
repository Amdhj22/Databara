package strava

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(srv *httptest.Server, tokens TokenSource) *Client {
	c := New("cid", "csec", tokens)
	c.BaseURL = srv.URL
	c.TokenURL = srv.URL + "/oauth/token"
	c.HTTPClient = srv.Client()
	return c
}

func TestClientDo_StatusMapping(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    error
	}{
		{"unauthorized", http.StatusUnauthorized, ErrUnauthorized},
		{"forbidden", http.StatusForbidden, ErrForbidden},
		{"not found", http.StatusNotFound, ErrNotFound},
		{"rate limited", http.StatusTooManyRequests, ErrRateLimited},
		{"server error", http.StatusInternalServerError, ErrUnexpectedStatus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			c := newTestClient(srv, newMemTokens(validToken()))
			err := c.do(context.Background(), http.MethodGet, "/x", nil, nil)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want errors.Is %v", err, tt.wantErr)
			}
		})
	}
}

func TestClientDo_AddsBearerToken(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, newMemTokens(validToken()))
	mustNoError(t, c.do(context.Background(), http.MethodGet, "/x", nil, &struct{}{}))

	if got != "Bearer access-1" {
		t.Errorf("Authorization header = %q, want Bearer access-1", got)
	}
}

func TestClientDo_RefreshesExpiredToken(t *testing.T) {
	var apiAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"access_token":"access-new",
				"refresh_token":"refresh-new",
				"expires_at": 9999999999
			}`))
		default:
			apiAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	tokens := newMemTokens(expiredToken())
	c := newTestClient(srv, tokens)

	mustNoError(t, c.do(context.Background(), http.MethodGet, "/x", nil, &struct{}{}))

	if apiAuth != "Bearer access-new" {
		t.Errorf("API call used %q, want Bearer access-new (post-refresh)", apiAuth)
	}
	cur, _ := tokens.Get(context.Background())
	if cur.AccessToken != "access-new" || cur.RefreshToken != "refresh-new" {
		t.Errorf("token store = %+v, want access-new/refresh-new", cur)
	}
	if tokens.saveCount != 1 {
		t.Errorf("saveCount = %d, want 1 (single refresh)", tokens.saveCount)
	}
}

func TestClientDo_DecodesJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"name":"morning ride","sport_type":"Ride"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, newMemTokens(validToken()))
	var got Activity
	mustNoError(t, c.do(context.Background(), http.MethodGet, "/activities/42", nil, &got))

	if got.ID != 42 || got.SportType != SportRide || !strings.Contains(got.Name, "morning") {
		t.Errorf("decoded = %+v, want id=42 / Ride / contains 'morning'", got)
	}
}
