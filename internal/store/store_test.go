package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveFlushRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := NewAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(UserDictFile, `{"かな":["仮名"]}`); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, UserDictFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"かな":["仮名"]}` {
		t.Fatalf("file = %s", got)
	}
	// A fresh store reads the persisted content back.
	s2, _ := NewAt(dir)
	if s2.Load(UserDictFile, "{}") != `{"かな":["仮名"]}` {
		t.Fatalf("reload = %q", s2.Load(UserDictFile, "{}"))
	}
}

func TestSaveRejectsInvalidJSON(t *testing.T) {
	s, _ := NewAt(t.TempDir())
	if err := s.Save(HistoryFile, "not json"); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestRemoveStaleTemps(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, ".skk-popup-abc123.tmp")
	keep := filepath.Join(dir, "userdict.json")
	other := filepath.Join(dir, ".hidden")
	for _, p := range []string{stale, keep, other} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := NewAt(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale temp not removed: %v", err)
	}
	for _, p := range []string{keep, other} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s should survive: %v", p, err)
		}
	}
}
