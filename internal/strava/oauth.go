package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultTokenURL is the OAuth2 token endpoint shared by code-exchange and
// refresh flows. Tests override Client.TokenURL to point elsewhere.
const DefaultTokenURL = "https://www.strava.com/oauth/token"

// tokenResponse is the wire format returned by /oauth/token. expires_at is a
// Unix timestamp; we convert to time.Time at the boundary so callers never
// see raw seconds-since-epoch.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

func (r tokenResponse) toTokenSet() TokenSet {
	return TokenSet{
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		ExpiresAt:    time.Unix(r.ExpiresAt, 0),
	}
}

// ExchangeCode swaps an authorization code (from the Strava OAuth redirect)
// for an initial TokenSet and persists it. Used once per athlete during
// onboarding.
func (c *Client) ExchangeCode(ctx context.Context, code string) (TokenSet, error) {
	form := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
	}
	tok, err := c.postTokenForm(ctx, form)
	if err != nil {
		return TokenSet{}, fmt.Errorf("exchange code: %w", err)
	}
	if err := c.Tokens.Save(ctx, tok); err != nil {
		return TokenSet{}, fmt.Errorf("persist initial token: %w", err)
	}
	return tok, nil
}

// Refresh exchanges a refresh_token for a fresh TokenSet. Strava rotates the
// refresh token on every call, so the new value MUST be persisted before the
// next API request. Refresh does not call Save itself — callers either go
// through Client.do (which persists for them) or persist explicitly.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (TokenSet, error) {
	form := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}
	tok, err := c.postTokenForm(ctx, form)
	if err != nil {
		return TokenSet{}, fmt.Errorf("refresh token: %w", err)
	}
	return tok, nil
}

// refreshAndStore is the do() helper used to transparently swap an expired
// token. Unexported because callers should rely on do() to handle expiry —
// using this directly would skip the expiry check.
func (c *Client) refreshAndStore(ctx context.Context, current TokenSet) (TokenSet, error) {
	fresh, err := c.Refresh(ctx, current.RefreshToken)
	if err != nil {
		return TokenSet{}, err
	}
	if err := c.Tokens.Save(ctx, fresh); err != nil {
		return TokenSet{}, fmt.Errorf("persist refreshed token: %w", err)
	}
	return fresh, nil
}

// postTokenForm POSTs an x-www-form-urlencoded payload to the OAuth endpoint
// and decodes the JSON response. The endpoint lives outside the REST API root
// so it does not go through Client.do().
func (c *Client) postTokenForm(ctx context.Context, form url.Values) (TokenSet, error) {
	endpoint := c.TokenURL
	if endpoint == "" {
		endpoint = DefaultTokenURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenSet{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return TokenSet{}, fmt.Errorf("oauth request: %w", err)
	}
	defer resp.Body.Close()

	if err := mapStatus(resp); err != nil {
		return TokenSet{}, err
	}
	var body tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return TokenSet{}, fmt.Errorf("decode token: %w", err)
	}
	return body.toTokenSet(), nil
}
