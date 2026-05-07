package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Amdhj22/databara/internal/strava"
)

// ErrNoToken is returned by FileTokenStore.Get when no token has been saved
// yet — typically the cold-start case before the operator has run the
// initial OAuth code exchange.
var ErrNoToken = errors.New("storage: no token saved")

// FileTokenStore persists a strava.TokenSet to a JSON file on disk.
//
// Writes are atomic: the new contents land in a temp file in the same
// directory and are renamed over the target. A SIGKILL or power loss in the
// middle of a Save leaves either the previous good copy or the new one,
// never a half-written file.
//
// File permissions are pinned to 0600 because the file holds OAuth refresh
// tokens. The parent directory is created (mode 0700) on first Save if it
// does not yet exist.
type FileTokenStore struct {
	Path string
	mu   sync.Mutex
}

// NewFileTokenStore returns a store that reads/writes path. The file does
// not need to exist yet; the first Get on a missing file returns ErrNoToken.
func NewFileTokenStore(path string) *FileTokenStore {
	return &FileTokenStore{Path: path}
}

// Compile-time check that *FileTokenStore satisfies the contract Strava's
// client expects. If TokenSource ever changes shape, this line breaks the
// build before the bad change can ship.
var _ strava.TokenSource = (*FileTokenStore)(nil)

// Get loads the persisted TokenSet. Returns ErrNoToken if nothing has been
// written yet, and a wrapped error for any other I/O or decode failure.
func (s *FileTokenStore) Get(ctx context.Context) (strava.TokenSet, error) {
	if err := ctx.Err(); err != nil {
		return strava.TokenSet{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return strava.TokenSet{}, ErrNoToken
	}
	if err != nil {
		return strava.TokenSet{}, fmt.Errorf("read token file: %w", err)
	}
	var tok strava.TokenSet
	if err := json.Unmarshal(data, &tok); err != nil {
		return strava.TokenSet{}, fmt.Errorf("decode token file: %w", err)
	}
	return tok, nil
}

// Save persists tok atomically (temp file + rename). The target file ends up
// at mode 0600.
func (s *FileTokenStore) Save(ctx context.Context, tok strava.TokenSet) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("ensure token dir: %w", err)
	}

	data, err := json.Marshal(tok)
	if err != nil {
		return fmt.Errorf("encode token: %w", err)
	}

	// CreateTemp gives 0600 by default, but Chmod after Write is belt-and-
	// suspenders: easier to spot-check than to rely on stdlib defaults.
	tmp, err := os.CreateTemp(dir, ".tokens-*.json")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, s.Path); err != nil {
		cleanup()
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}
