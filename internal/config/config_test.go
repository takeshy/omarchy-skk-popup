package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFromDefaults(t *testing.T) {
	cfg := LoadFrom(filepath.Join(t.TempDir(), "missing.toml"))
	if !cfg.Clipboard.AutoPaste || cfg.Clipboard.AutoPasteDelayMs != 80 || cfg.Clipboard.PasteKey != "ctrl+shift+v" {
		t.Fatalf("defaults not applied: %+v", cfg.Clipboard)
	}
}

func TestAutoPasteParsing(t *testing.T) {
	cases := map[string]bool{
		"auto_paste = false":    false,
		"auto_paste = 0":        false,
		`auto_paste = "true"`:   true,
		"auto_paste = 1":        true,
		"auto_paste = FALSE":    false,
		"auto_paste = notabool": true, // unparseable: keep the default (true)
	}
	for body, want := range cases {
		cfg := LoadFrom(writeConfig(t, "[clipboard]\n"+body+"\n"))
		if cfg.Clipboard.AutoPaste != want {
			t.Errorf("%q: AutoPaste = %v, want %v", body, cfg.Clipboard.AutoPaste, want)
		}
	}
}

func TestDictionaryPathsAndComments(t *testing.T) {
	home, _ := os.UserHomeDir()
	cfg := LoadFrom(writeConfig(t, `
[clipboard]
paste_key = "ctrl+v"  # inline comment
auto_paste_delay_ms = 120

[dictionary]
dir = "~/.skk"
external_path = "/tmp/SKK-JISYO.user"
`))
	if cfg.Clipboard.PasteKey != "ctrl+v" || cfg.Clipboard.AutoPasteDelayMs != 120 {
		t.Fatalf("clipboard = %+v", cfg.Clipboard)
	}
	if cfg.Dictionary.Dir != filepath.Join(home, ".skk") {
		t.Fatalf("dir = %q", cfg.Dictionary.Dir)
	}
	if cfg.Dictionary.ExternalPath != "/tmp/SKK-JISYO.user" {
		t.Fatalf("external_path = %q", cfg.Dictionary.ExternalPath)
	}
}

func TestInvalidPasteKeyIgnored(t *testing.T) {
	cfg := LoadFrom(writeConfig(t, "[clipboard]\npaste_key = \"ctrl+alt+v\"\n"))
	if cfg.Clipboard.PasteKey != "ctrl+shift+v" {
		t.Fatalf("bad paste_key should fall back to default, got %q", cfg.Clipboard.PasteKey)
	}
}
