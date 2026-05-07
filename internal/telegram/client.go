package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultBaseURL is the public Bot API endpoint. Tests override it to point
// at an httptest.Server.
const DefaultBaseURL = "https://api.telegram.org"

// Client talks to the Telegram Bot HTTP API. It is safe for concurrent use;
// the http.Client owns its own connection pool.
//
// Token is interpolated into every request URL (Telegram's standard pattern
// `/bot<TOKEN>/<method>`) — a deliberate choice from the platform, not a
// secret-handling oversight. Keep it out of logs.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// New returns a Client configured for the public Telegram endpoint with a
// 10-second per-request timeout. Replace BaseURL/HTTPClient on the returned
// value for tests or custom transport.
func New(token string) *Client {
	return &Client{
		BaseURL:    DefaultBaseURL,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// do executes a Bot API call with a JSON body and decodes the wrapped
// response into out (which may be nil to discard the result). It maps the
// envelope's OK==false case onto the sentinel errors in errors.go.
//
// HTTP-level transport failures (DNS, timeout, refused) bubble up as plain
// wrapped errors; only Telegram's structured failures are mapped.
func (c *Client) do(ctx context.Context, method string, in any, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshal %s body: %w", method, err)
		}
		body = bytes.NewReader(b)
	}

	url := c.BaseURL + "/bot" + c.Token + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("build %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram %s: %w", method, err)
	}
	defer resp.Body.Close()

	// Telegram responses are tiny (a few KiB at most). 4 MiB is a defensive
	// ceiling that prevents a misbehaving proxy from streaming garbage.
	var apiResp apiResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&apiResp); err != nil {
		return fmt.Errorf("decode %s response (status=%d): %w", method, resp.StatusCode, err)
	}
	if !apiResp.OK {
		return mapError(apiResp)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(apiResp.Result, out)
}
