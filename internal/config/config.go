// Package config loads ~/.config/skk-popup/config.toml.
//
// The file is shared with the Wails skk-popup daemon; only the keys the
// engine needs are read ([clipboard] and [dictionary]). A small TOML
// subset is supported: [section] headers and `key = value` pairs where
// value is a string ("..."), integer, or boolean. Unknown keys are ignored.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ClipboardConfig struct {
	AutoPaste        bool
	AutoPasteDelayMs int
	PasteKey         string // "ctrl+v" | "ctrl+shift+v"
}

type DictionaryConfig struct {
	// Dir holds SKK-JISYO files loaded in name order after the bundled
	// list. Defaults to $XDG_DATA_HOME/skk-popup/dict.
	Dir string
	// ExternalPath is one extra dictionary file (SKK-JISYO or JSON).
	ExternalPath string
}

type Config struct {
	Clipboard  ClipboardConfig
	Dictionary DictionaryConfig
}

func Default() *Config {
	return &Config{
		Clipboard: ClipboardConfig{
			AutoPaste:        true,
			AutoPasteDelayMs: 80,
			PasteKey:         "ctrl+shift+v",
		},
	}
}

// Dir returns $XDG_CONFIG_HOME/skk-popup (or ~/.config/skk-popup).
func Dir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "skk-popup")
}

func Path() string {
	dir := Dir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.toml")
}

func Load() *Config {
	path := Path()
	if path == "" {
		return Default()
	}
	return LoadFrom(path)
}

// LoadFrom parses the TOML subset at path on top of the defaults. A
// missing or unreadable file yields the defaults.
func LoadFrom(path string) *Config {
	cfg := Default()
	file, err := os.Open(path)
	if err != nil {
		return cfg
	}
	defer file.Close()

	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		value := stripInlineComment(strings.TrimSpace(line[eq+1:]))
		if len(value) >= 2 && strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			value = value[1 : len(value)-1]
		}
		cfg.apply(section, key, value)
	}
	return cfg
}

func stripInlineComment(value string) string {
	inString := false
	escaped := false
	for i, r := range value {
		if inString && r == '\\' && !escaped {
			escaped = true
			continue
		}
		if r == '"' && !escaped {
			inString = !inString
		} else if r == '#' && !inString {
			return strings.TrimSpace(value[:i])
		}
		escaped = false
	}
	return strings.TrimSpace(value)
}

func (c *Config) apply(section, key, value string) {
	switch section {
	case "clipboard":
		switch key {
		case "auto_paste":
			if b, err := strconv.ParseBool(value); err == nil {
				c.Clipboard.AutoPaste = b
			}
		case "auto_paste_delay_ms":
			if n, err := strconv.Atoi(value); err == nil {
				c.Clipboard.AutoPasteDelayMs = n
			}
		case "paste_key":
			if value == "ctrl+v" || value == "ctrl+shift+v" {
				c.Clipboard.PasteKey = value
			}
		}
	case "dictionary":
		switch key {
		case "dir":
			c.Dictionary.Dir = expandHome(value)
		case "external_path":
			c.Dictionary.ExternalPath = expandHome(value)
		}
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
