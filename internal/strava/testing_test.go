package strava

import (
	"context"
	"sync"
	"testing"
	"time"
)

// memTokens is a TokenSource backed by an in-memory value, used by tests.
// It records every Save call so assertions can verify rotation.
type memTokens struct {
	mu        sync.Mutex
	cur       TokenSet
	saveCount int
}

func newMemTokens(initial TokenSet) *memTokens {
	return &memTokens{cur: initial}
}

func (m *memTokens) Get(_ context.Context) (TokenSet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cur, nil
}

func (m *memTokens) Save(_ context.Context, tok TokenSet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cur = tok
	m.saveCount++
	return nil
}

// validToken returns a TokenSet that is not expired for ~1h.
func validToken() TokenSet {
	return TokenSet{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}
}

// expiredToken returns a TokenSet whose access token is already past expiry.
func expiredToken() TokenSet {
	return TokenSet{
		AccessToken:  "access-old",
		RefreshToken: "refresh-old",
		ExpiresAt:    time.Now().Add(-1 * time.Minute),
	}
}

// mustNoError fails the test immediately if err is non-nil.
func mustNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
