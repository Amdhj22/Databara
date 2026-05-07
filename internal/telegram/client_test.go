package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(srv *httptest.Server) *Client {
	c := New("TEST-TOKEN")
	c.BaseURL = srv.URL
	c.HTTPClient = srv.Client()
	return c
}

func okResponse() []byte {
	body, _ := json.Marshal(map[string]any{
		"ok":     true,
		"result": map[string]any{"message_id": 7, "date": 1700000000},
	})
	return body
}

func errResponse(code int, description string, retryAfter int) []byte {
	payload := map[string]any{
		"ok":          false,
		"error_code":  code,
		"description": description,
	}
	if retryAfter > 0 {
		payload["parameters"] = map[string]any{"retry_after": retryAfter}
	}
	body, _ := json.Marshal(payload)
	return body
}

func TestSendMessage_HappyPath(t *testing.T) {
	var seenPath string
	var seenContentType string
	var seenBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &seenBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(okResponse())
	}))
	defer srv.Close()

	c := newTestClient(srv)
	if err := c.SendMessage(context.Background(), 12345, "hello"); err != nil {
		t.Fatalf("SendMessage() error: %v", err)
	}

	if seenPath != "/botTEST-TOKEN/sendMessage" {
		t.Errorf("path = %q, want /botTEST-TOKEN/sendMessage", seenPath)
	}
	if !strings.HasPrefix(seenContentType, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", seenContentType)
	}
	if int64(seenBody["chat_id"].(float64)) != 12345 {
		t.Errorf("chat_id = %v, want 12345", seenBody["chat_id"])
	}
	if seenBody["text"] != "hello" {
		t.Errorf("text = %v, want hello", seenBody["text"])
	}
	if _, hasParseMode := seenBody["parse_mode"]; hasParseMode {
		t.Error("parse_mode should be omitted by default")
	}
}

func TestSendMessage_AppliesOptions(t *testing.T) {
	var seenBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &seenBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(okResponse())
	}))
	defer srv.Close()

	c := newTestClient(srv)
	err := c.SendMessage(context.Background(), 1, "x",
		WithParseMode("MarkdownV2"),
		WithSilent(),
		WithoutPreview(),
	)
	if err != nil {
		t.Fatalf("SendMessage() error: %v", err)
	}
	if seenBody["parse_mode"] != "MarkdownV2" {
		t.Errorf("parse_mode = %v, want MarkdownV2", seenBody["parse_mode"])
	}
	if seenBody["disable_notification"] != true {
		t.Errorf("disable_notification = %v, want true", seenBody["disable_notification"])
	}
	if seenBody["disable_web_page_preview"] != true {
		t.Errorf("disable_web_page_preview = %v, want true", seenBody["disable_web_page_preview"])
	}
}

func TestSendMessage_RejectsEmptyText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("API should not be hit when text is empty")
	}))
	defer srv.Close()

	c := newTestClient(srv)
	if err := c.SendMessage(context.Background(), 1, "  \n\t"); err == nil {
		t.Fatal("expected error for whitespace-only text")
	}
}

func TestSendMessage_MapsErrorCodes(t *testing.T) {
	tests := []struct {
		name         string
		code         int
		wantSentinel error
	}{
		{"bad request", 400, ErrBadRequest},
		{"unauthorized", 401, ErrUnauthorized},
		{"forbidden", 403, ErrForbidden},
		{"not found", 404, ErrNotFound},
		{"conflict", 409, ErrConflict},
		{"server error", 500, ErrServerError},
		{"unknown", 418, ErrAPIError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(errResponse(tt.code, "boom", 0))
			}))
			defer srv.Close()

			c := newTestClient(srv)
			err := c.SendMessage(context.Background(), 1, "x")
			if !errors.Is(err, tt.wantSentinel) {
				t.Fatalf("err = %v, want errors.Is %v", err, tt.wantSentinel)
			}
		})
	}
}

func TestSendMessage_RateLimitSurfacesRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(errResponse(429, "Too Many Requests", 30))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	err := c.SendMessage(context.Background(), 1, "x")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited via errors.Is", err)
	}
	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("errors.As(*RateLimitError) failed for %v", err)
	}
	if rle.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v, want 30s", rle.RetryAfter)
	}
	if !strings.Contains(rle.Description, "Many Requests") {
		t.Errorf("Description = %q, want it to mention 'Many Requests'", rle.Description)
	}
}
