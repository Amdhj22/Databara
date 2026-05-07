package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

func newTestClient(srv *httptest.Server) *Client {
	return New("test-key", "claude-sonnet-4-6",
		option.WithBaseURL(srv.URL),
		option.WithHTTPClient(srv.Client()),
	)
}

// stubResponse returns a minimal Messages API response shape with a single
// text block containing the supplied text.
func stubResponse(text string) []byte {
	body, _ := json.Marshal(map[string]any{
		"id":    "msg_test",
		"type":  "message",
		"role":  "assistant",
		"model": "claude-sonnet-4-6",
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":  120,
			"output_tokens": 25,
		},
	})
	return body
}

func TestComment_HappyPath(t *testing.T) {
	var seenBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1/messages") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &seenBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(stubResponse("좋은 페이스로 달리셨네요!"))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	got, err := c.Comment(context.Background(), CommentRequest{
		Sport:   "Run",
		Summary: "Distance: 10.0 km\nMoving time: 50:00\nAvg HR: 152",
	})
	if err != nil {
		t.Fatalf("Comment() error: %v", err)
	}
	if !strings.Contains(got, "좋은 페이스") {
		t.Errorf("got = %q, want it to contain '좋은 페이스'", got)
	}

	if seenBody["model"] != "claude-sonnet-4-6" {
		t.Errorf("request model = %v, want claude-sonnet-4-6", seenBody["model"])
	}
	if mt, _ := seenBody["max_tokens"].(float64); int(mt) != maxCommentTokens {
		t.Errorf("max_tokens = %v, want %d", seenBody["max_tokens"], maxCommentTokens)
	}
}

func TestComment_RejectsEmptySummary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("API should not be hit when Summary is empty")
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.Comment(context.Background(), CommentRequest{Sport: "Run"})
	if err == nil {
		t.Fatal("expected error for empty Summary")
	}
}

func TestComment_SendsCacheControlOnSystem(t *testing.T) {
	var seenBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &seenBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(stubResponse("ok"))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.Comment(context.Background(), CommentRequest{
		Sport: "Ride", Summary: "Distance: 30 km",
	})
	if err != nil {
		t.Fatalf("Comment() error: %v", err)
	}

	sys, ok := seenBody["system"].([]any)
	if !ok || len(sys) == 0 {
		t.Fatalf("system block missing from request body: %v", seenBody["system"])
	}
	first, _ := sys[0].(map[string]any)
	cc, _ := first["cache_control"].(map[string]any)
	if cc["type"] != "ephemeral" {
		t.Errorf("cache_control = %v, want type=ephemeral; got %v", cc, cc)
	}
}

func TestComment_PropagatesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid"}}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.Comment(context.Background(), CommentRequest{
		Sport: "Run", Summary: "Distance: 5 km",
	})
	if err == nil {
		t.Fatal("expected error from 401")
	}
}

func TestExtractText_NoBlocks(t *testing.T) {
	if _, err := extractText(&anthropic.Message{}); err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestFormatUserMessage_DefaultsSport(t *testing.T) {
	got := formatUserMessage(CommentRequest{Summary: "x"})
	if !strings.HasPrefix(got, "Sport: Workout\n") {
		t.Errorf("default sport not applied: %q", got)
	}
}
