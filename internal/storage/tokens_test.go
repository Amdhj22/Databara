package storage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Amdhj22/databara/internal/strava"
)

func mustNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFileTokenStore_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	store := NewFileTokenStore(path)

	want := strava.TokenSet{
		AccessToken:  "a",
		RefreshToken: "r",
		ExpiresAt:    time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
	}

	mustNoError(t, store.Save(context.Background(), want))

	got, err := store.Get(context.Background())
	mustNoError(t, err)

	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken {
		t.Errorf("token mismatch: got %+v, want %+v", got, want)
	}
	if !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, want.ExpiresAt)
	}
}

func TestFileTokenStore_Get_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.json")
	store := NewFileTokenStore(path)

	_, err := store.Get(context.Background())
	if !errors.Is(err, ErrNoToken) {
		t.Fatalf("err = %v, want errors.Is %v", err, ErrNoToken)
	}
}

func TestFileTokenStore_Save_CreatesParentDir(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "deep", "nested", "tokens.json")
	store := NewFileTokenStore(path)

	mustNoError(t, store.Save(context.Background(), strava.TokenSet{AccessToken: "x"}))

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist after Save: %v", err)
	}
}

func TestFileTokenStore_Save_OverwritesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	store := NewFileTokenStore(path)
	ctx := context.Background()

	mustNoError(t, store.Save(ctx, strava.TokenSet{AccessToken: "first"}))
	mustNoError(t, store.Save(ctx, strava.TokenSet{AccessToken: "second"}))

	got, err := store.Get(ctx)
	mustNoError(t, err)

	if got.AccessToken != "second" {
		t.Errorf("AccessToken = %q, want second", got.AccessToken)
	}
}

func TestFileTokenStore_Save_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission semantics differ on windows")
	}
	path := filepath.Join(t.TempDir(), "tokens.json")
	store := NewFileTokenStore(path)

	mustNoError(t, store.Save(context.Background(), strava.TokenSet{AccessToken: "x"}))

	info, err := os.Stat(path)
	mustNoError(t, err)
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode = %o, want 0600", mode)
	}
}

func TestFileTokenStore_Get_CorruptedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	store := NewFileTokenStore(path)

	_, err := store.Get(context.Background())
	if err == nil {
		t.Fatal("expected error for corrupted JSON")
	}
	if errors.Is(err, ErrNoToken) {
		t.Errorf("corrupted JSON should NOT match ErrNoToken; got %v", err)
	}
}

func TestFileTokenStore_Save_HonorsCanceledContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	store := NewFileTokenStore(path)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.Save(ctx, strava.TokenSet{AccessToken: "x"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Errorf("file should not have been written under canceled ctx")
	}
}

func TestFileTokenStore_Get_HonorsCanceledContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	store := NewFileTokenStore(path)
	mustNoError(t, store.Save(context.Background(), strava.TokenSet{AccessToken: "x"}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := store.Get(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestFileTokenStore_OnDiskFormatIsJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	store := NewFileTokenStore(path)
	mustNoError(t, store.Save(context.Background(), strava.TokenSet{
		AccessToken:  "abc",
		RefreshToken: "def",
		ExpiresAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}))

	raw, err := os.ReadFile(path)
	mustNoError(t, err)

	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("on-disk format should be JSON: %v\nraw: %s", err, raw)
	}
	if generic["access_token"] != "abc" {
		t.Errorf("access_token field missing/wrong: %v", generic)
	}
	if generic["refresh_token"] != "def" {
		t.Errorf("refresh_token field missing/wrong: %v", generic)
	}
}

func TestFileTokenStore_Save_LeavesNoTempLeak(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")
	store := NewFileTokenStore(path)

	mustNoError(t, store.Save(context.Background(), strava.TokenSet{AccessToken: "x"}))

	entries, err := os.ReadDir(dir)
	mustNoError(t, err)
	for _, e := range entries {
		if e.Name() != "tokens.json" {
			t.Errorf("unexpected leftover entry: %q", e.Name())
		}
	}
}
