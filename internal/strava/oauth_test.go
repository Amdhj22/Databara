package strava

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestExchangeCode_PersistsToken(t *testing.T) {
	var seenForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seenForm, _ = url.ParseQuery(string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token":"a1",
			"refresh_token":"r1",
			"expires_at": 2000000000
		}`))
	}))
	defer srv.Close()

	tokens := newMemTokens(TokenSet{})
	c := New("cid", "csec", tokens)
	c.TokenURL = srv.URL
	c.HTTPClient = srv.Client()

	tok, err := c.ExchangeCode(context.Background(), "auth-code-123")
	mustNoError(t, err)

	if seenForm.Get("grant_type") != "authorization_code" {
		t.Errorf("grant_type = %q, want authorization_code", seenForm.Get("grant_type"))
	}
	if seenForm.Get("code") != "auth-code-123" {
		t.Errorf("code = %q, want auth-code-123", seenForm.Get("code"))
	}
	if tok.AccessToken != "a1" || tok.RefreshToken != "r1" {
		t.Errorf("returned token = %+v, want a1/r1", tok)
	}

	stored, _ := tokens.Get(context.Background())
	if stored.AccessToken != "a1" {
		t.Errorf("token not persisted, store = %+v", stored)
	}
	wantExpiry := time.Unix(2000000000, 0)
	if !tok.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("ExpiresAt = %v, want %v", tok.ExpiresAt, wantExpiry)
	}
}

func TestRefresh_ReturnsRotatedToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		if form.Get("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", form.Get("grant_type"))
		}
		if form.Get("refresh_token") != "old-refresh" {
			t.Errorf("refresh_token = %q, want old-refresh", form.Get("refresh_token"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token":"new-access",
			"refresh_token":"new-refresh",
			"expires_at": 2000000000
		}`))
	}))
	defer srv.Close()

	c := New("cid", "csec", newMemTokens(TokenSet{}))
	c.TokenURL = srv.URL
	c.HTTPClient = srv.Client()

	tok, err := c.Refresh(context.Background(), "old-refresh")
	mustNoError(t, err)

	if tok.AccessToken != "new-access" || tok.RefreshToken != "new-refresh" {
		t.Errorf("rotated token = %+v, want new-access/new-refresh", tok)
	}
}

func TestPostTokenForm_ContentType(t *testing.T) {
	var ct string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct = r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`{"access_token":"x","refresh_token":"y","expires_at":1}`))
	}))
	defer srv.Close()

	c := New("cid", "csec", newMemTokens(TokenSet{}))
	c.TokenURL = srv.URL
	c.HTTPClient = srv.Client()

	_, err := c.postTokenForm(context.Background(), url.Values{"k": {"v"}})
	mustNoError(t, err)

	if !strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
		t.Errorf("Content-Type = %q, want form-urlencoded", ct)
	}
}
