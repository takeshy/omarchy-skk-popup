// Package store persists the user dictionary, candidate history, and
// input history as JSON files under $XDG_DATA_HOME/skk-popup. The layout
// and file names are shared with the Wails skk-popup daemon, so both
// front ends read and write the same data.
//
// Writes are debounced (flushed 2 seconds after the last update) and
// always flushed when the popup hides. All writes go through a temporary
// file + rename so a crash never leaves truncated files behind.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	UserDictFile     = "userdict.json"
	HistoryFile      = "history.json"
	InputHistoryFile = "input-history.json"
	flushInterval    = 2 * time.Second
)

type Store struct {
	mu    sync.Mutex
	dir   string
	files map[string]*staged
	timer *time.Timer
}

type staged struct {
	content string
	loaded  bool
	dirty   bool
}

// DataDir returns $XDG_DATA_HOME/skk-popup (or ~/.local/share/skk-popup).
func DataDir() string {
	if base := os.Getenv("XDG_DATA_HOME"); base != "" {
		return filepath.Join(base, "skk-popup")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "skk-popup")
}

// New creates a store rooted at DataDir().
func New() (*Store, error) { return NewAt(DataDir()) }

// NewAt creates a store rooted at an explicit directory.
func NewAt(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("store: no data directory")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	removeStaleTemps(dir)
	return &Store{dir: dir, files: map[string]*staged{}}, nil
}

// removeStaleTemps deletes writeAtomic scratch files a previous crash left
// between CreateTemp and Rename. Best effort: errors are ignored.
func removeStaleTemps(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasPrefix(name, ".skk-popup-") && strings.HasSuffix(name, ".tmp") {
			os.Remove(filepath.Join(dir, name))
		}
	}
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) Load(name, empty string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.files[name]
	if entry == nil {
		entry = &staged{}
		s.files[name] = entry
	}
	if !entry.loaded {
		entry.content = readFileOrEmpty(filepath.Join(s.dir, name), empty)
		entry.loaded = true
	}
	return entry.content
}

// Save stages a JSON document; it is flushed after the debounce interval.
func (s *Store) Save(name, data string) error {
	if !json.Valid([]byte(data)) {
		return os.ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.files[name]
	if entry == nil {
		entry = &staged{}
		s.files[name] = entry
	}
	entry.content = data
	entry.loaded = true
	entry.dirty = true
	s.scheduleFlushLocked()
	return nil
}

// Flush writes staged updates now. Failed writes stay dirty and are retried.
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	var errs []error
	for name, entry := range s.files {
		if !entry.dirty {
			continue
		}
		if err := writeAtomic(filepath.Join(s.dir, name), entry.content); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		entry.dirty = false
	}
	if len(errs) > 0 {
		s.scheduleFlushLocked()
	}
	return errors.Join(errs...)
}

func (s *Store) scheduleFlushLocked() {
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(flushInterval, func() {
		if err := s.Flush(); err != nil {
			log.Printf("store flush: %v", err)
		}
	})
}

func readFileOrEmpty(path, empty string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return empty
	}
	return string(data)
}

func writeAtomic(path, content string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".skk-popup-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}
