package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultBaseURL is the public Strava REST API root. Tests override it to
// point at an httptest.Server.
const DefaultBaseURL = "https://www.strava.com/api/v3"

// TokenSource abstracts where tokens are persisted. The Strava client only
// reads/writes through this interface so storage choices (in-memory, file,
// SQLite) stay independent.
type TokenSource interface {
	Get(ctx context.Context) (TokenSet, error)
	Save(ctx context.Context, tok TokenSet) error
}

// Client talks to the Strava REST API. It is safe for concurrent use.
//
// The client refreshes expired access tokens transparently before each
// request. ClientID/ClientSecret are required for the refresh flow even
// though most call sites only need the access token.
//
// BaseURL/TokenURL are split because the REST API and OAuth endpoint live
// under different hosts (`/api/v3` vs. `/oauth/token`). Tests override both.
type Client struct {
	BaseURL      string
	TokenURL     string
	HTTPClient   *http.Client
	ClientID     string
	ClientSecret string
	Tokens       TokenSource
}

// New returns a Client wired to the public Strava endpoints with a sane HTTP
// timeout. Replace BaseURL/TokenURL/HTTPClient on the returned value for tests.
func New(clientID, clientSecret string, tokens TokenSource) *Client {
	return &Client{
		BaseURL:      DefaultBaseURL,
		TokenURL:     DefaultTokenURL,
		HTTPClient:   &http.Client{Timeout: 15 * time.Second},
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Tokens:       tokens,
	}
}

// do executes an authenticated request against the Strava API and decodes the
// response body into out (which may be nil to discard). Non-2xx responses are
// mapped to sentinel errors so callers can branch with errors.Is.
//
// Token refresh is handled here: if the cached token is expired we refresh
// before issuing the request. We do NOT retry on a 401, because by the time
// Strava returns 401 the refresh-token-rotation may already be racing another
// caller; let the next call see the freshly persisted token instead.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, out any) error {
	tok, err := c.Tokens.Get(ctx)
	if err != nil {
		return fmt.Errorf("load token: %w", err)
	}
	if tok.Expired() {
		if tok, err = c.refreshAndStore(ctx, tok); err != nil {
			return err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("strava request: %w", err)
	}
	defer resp.Body.Close()

	if err := mapStatus(resp); err != nil {
		return err
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// mapStatus turns HTTP response codes into the package's sentinel errors.
// Any non-2xx response drains the body so the connection can be reused.
func mapStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %s", ErrUnauthorized, body)
	case http.StatusForbidden:
		return fmt.Errorf("%w: %s", ErrForbidden, body)
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, body)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ErrRateLimited, body)
	default:
		return fmt.Errorf("%w: %d %s", ErrUnexpectedStatus, resp.StatusCode, body)
	}
}
