// skk-popup-engine is the headless SKK engine behind the Omarchy shell
// plugin. The QML panel spawns `skk-popup-engine serve` and exchanges
// JSON lines with it over stdin/stdout: every request is answered with a
// full render state (see internal/skk.State), so the panel is a pure view.
//
//	skk-popup-engine serve        speak the JSON-lines protocol (used by Panel.qml)
//	skk-popup-engine dict fetch   download SKK-JISYO.* into the dictionary dir
//	skk-popup-engine dict list    print the dictionaries that would be loaded
//	skk-popup-engine version
package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/takeshy/omarchy-skk-popup/internal/clipboard"
	"github.com/takeshy/omarchy-skk-popup/internal/config"
	"github.com/takeshy/omarchy-skk-popup/internal/skk"
	"github.com/takeshy/omarchy-skk-popup/internal/store"
)

var version = "dev"

const dictionaryBaseURL = "https://skk-dev.github.io/dict"

// bundledDictionaries are fetched by `dict fetch` and loaded first, in
// this order, so the large dictionary's candidates come before the
// specialised ones (same order as skk-popup's dictionary_sources.json).
var bundledDictionaries = []string{
	"SKK-JISYO.L",
	"SKK-JISYO.geo",
	"SKK-JISYO.jinmei",
	"SKK-JISYO.propernoun",
	"SKK-JISYO.station",
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("skk-popup-engine: ")
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "serve":
		if err := serve(); err != nil {
			log.Fatal(err)
		}
	case "dict":
		if err := dictCommand(args[1:]); err != nil {
			log.Fatal(err)
		}
	case "version", "--version", "-v":
		fmt.Println("skk-popup-engine", version)
	case "-h", "--help", "help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `skk-popup-engine - SKK engine for the skk-popup Omarchy plugin

Usage:
  skk-popup-engine serve        JSON-lines protocol on stdin/stdout (used by Panel.qml)
  skk-popup-engine dict fetch   download SKK-JISYO.* into the dictionary directory
  skk-popup-engine dict list    print the dictionary files that serve would load
  skk-popup-engine version`)
}

// ---- dictionaries ----------------------------------------------------------

func dictionaryDir(cfg *config.Config) string {
	if cfg.Dictionary.Dir != "" {
		return cfg.Dictionary.Dir
	}
	return filepath.Join(store.DataDir(), "dict")
}

// dictionaryFiles lists the files serve loads: the bundled names that
// exist in the dictionary dir (in bundled order), any other file in that
// dir (sorted), then the configured external path.
func dictionaryFiles(cfg *config.Config) []string {
	dir := dictionaryDir(cfg)
	var files []string
	seen := map[string]bool{}
	for _, name := range bundledDictionaries {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			files = append(files, path)
			seen[name] = true
		}
	}
	if entries, err := os.ReadDir(dir); err == nil {
		var extra []string
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || seen[name] || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".gz") || strings.HasSuffix(name, ".tmp") {
				continue
			}
			extra = append(extra, filepath.Join(dir, name))
		}
		sort.Strings(extra)
		files = append(files, extra...)
	}
	if cfg.Dictionary.ExternalPath != "" {
		files = append(files, cfg.Dictionary.ExternalPath)
	}
	return files
}

func dictCommand(args []string) error {
	cfg := config.Load()
	if len(args) == 0 {
		return errors.New("dict: expected fetch or list")
	}
	switch args[0] {
	case "list":
		files := dictionaryFiles(cfg)
		if len(files) == 0 {
			fmt.Printf("no dictionaries in %s (run: skk-popup-engine dict fetch)\n", dictionaryDir(cfg))
			return nil
		}
		for _, f := range files {
			fmt.Println(f)
		}
		return nil
	case "fetch":
		names := args[1:]
		if len(names) == 0 {
			names = bundledDictionaries
		}
		dir := dictionaryDir(cfg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		for _, name := range names {
			if err := fetchDictionary(dir, name); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
		return nil
	}
	return fmt.Errorf("dict: unknown subcommand %q", args[0])
}

func fetchDictionary(dir, name string) error {
	src := dictionaryBaseURL + "/" + url.PathEscape(name) + ".gz"
	fmt.Fprintf(os.Stderr, "fetching %s\n", src)
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(src)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()

	tmp, err := os.CreateTemp(dir, "."+name+"-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	n, err := io.Copy(tmp, gz)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(tmpPath)
		return err
	}
	dest := filepath.Join(dir, name)
	if err := os.Rename(tmpPath, dest); err != nil {
		os.Remove(tmpPath)
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes)\n", dest, n)
	return nil
}

// ---- serve -----------------------------------------------------------------

type request struct {
	Op     string `json:"op"`
	Key    string `json:"key"`
	Ctrl   bool   `json:"ctrl"`
	Shift  bool   `json:"shift"`
	Alt    bool   `json:"alt"`
	Pos    int    `json:"pos"`
	Anchor int    `json:"anchor"` // setSelection: drag start offset
	Path   string `json:"path"`   // addDict / removeDict
}

func expandUser(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func parseStringList(raw string) []string {
	var list []string
	if json.Unmarshal([]byte(raw), &list) != nil {
		return nil
	}
	out := list[:0]
	for _, s := range list {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func marshalStringList(list []string) string {
	if list == nil {
		list = []string{}
	}
	data, err := json.Marshal(list)
	if err != nil {
		return "[]"
	}
	return string(data)
}

type storePersister struct{ s *store.Store }

func (p storePersister) SaveUserDict(data string)     { p.save(store.UserDictFile, data) }
func (p storePersister) SaveHistory(data string)      { p.save(store.HistoryFile, data) }
func (p storePersister) SaveInputHistory(data string) { p.save(store.InputHistoryFile, data) }
func (p storePersister) save(name, data string) {
	if p.s == nil {
		return
	}
	if err := p.s.Save(name, data); err != nil {
		log.Printf("save %s: %v", name, err)
	}
}

func serve() error {
	cfg := config.Load()
	out := bufio.NewWriter(os.Stdout)
	emit := func(v any) {
		data, err := json.Marshal(v)
		if err != nil {
			return
		}
		out.Write(data)
		out.WriteByte('\n')
		out.Flush()
	}

	st, err := store.New()
	if err != nil {
		log.Printf("store: %v (user dictionary will not persist)", err)
		st = nil
	}

	dict := skk.NewDictionary()
	var loaded []string
	loadDict := func(path string) bool {
		if err := dict.LoadSystemFile(expandUser(path)); err != nil {
			log.Printf("dictionary %s: %v", path, err)
			emit(map[string]any{"type": "error", "message": fmt.Sprintf("dictionary %s: %v", filepath.Base(path), err)})
			return false
		}
		loaded = append(loaded, path)
		return true
	}
	for _, path := range dictionaryFiles(cfg) {
		loadDict(path)
	}
	// Extra dictionaries added from the panel Settings.
	var extraDicts []string
	if st != nil {
		extraDicts = parseStringList(st.Load(store.ExtraDictsFile, "[]"))
	}
	kept := extraDicts[:0]
	for _, path := range extraDicts {
		if loadDict(path) {
			kept = append(kept, path)
		}
	}
	extraDicts = kept
	if len(loaded) == 0 {
		log.Printf("no system dictionary found in %s (run: skk-popup-engine dict fetch)", dictionaryDir(cfg))
		emit(map[string]any{"type": "error", "message": "No dictionary. Run: skk-popup-engine dict fetch"})
	}

	clip := &clipboard.Wayland{}
	engine := skk.New(dict, clip, storePersister{st})
	if st != nil {
		dict.SetUserJSON(st.Load(store.UserDictFile, "{}"))
		dict.SetHistoryJSON(st.Load(store.HistoryFile, "{}"))
		engine.SetInputHistoryJSON(st.Load(store.InputHistoryFile, "[]"))
	}

	exePath, _ := os.Executable()
	emit(map[string]any{
		"type":         "ready",
		"version":      version,
		"entries":      dict.SystemEntries(),
		"dictionaries": loaded,
		"extraDicts":   extraDicts,
		"dataDir":      store.DataDir(),
		"configPath":   config.Path(),
		"enginePath":   exePath,
	})
	emit(stateMessage(engine.State()))

	emitConfig := func() {
		emit(map[string]any{
			"type":       "config",
			"entries":    dict.SystemEntries(),
			"extraDicts": extraDicts,
		})
	}

	pasteArmed := false
	// The auto-paste shortcut fires a short delay after the popup hides, on
	// its own goroutine so the request loop stays responsive; pasteWG lets
	// serve() wait for a paste in flight before it returns.
	var pasteWG sync.WaitGroup
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			emit(map[string]any{"type": "error", "message": "bad request: " + err.Error()})
			continue
		}
		engine.ClearFlags()
		switch req.Op {
		case "key":
			engine.HandleKey(skk.Key{Key: req.Key, Ctrl: req.Ctrl, Shift: req.Shift, Alt: req.Alt})
		case "shown":
			engine.Shown()
		case "hidden":
			if st != nil {
				if err := st.Flush(); err != nil {
					log.Printf("flush: %v", err)
				}
			}
			if pasteArmed {
				pasteArmed = false
				delay := time.Duration(cfg.Clipboard.AutoPasteDelayMs) * time.Millisecond
				key := cfg.Clipboard.PasteKey
				pasteWG.Add(1)
				go func() {
					defer pasteWG.Done()
					if delay > 0 {
						time.Sleep(delay)
					}
					if err := clip.Paste(key); err != nil {
						log.Printf("paste: %v", err)
					}
				}()
			}
		case "copy":
			engine.Copy()
		case "close":
			engine.RequestClose()
		case "paste":
			engine.PasteClipboard()
		case "toggleMode":
			engine.ToggleMode()
		case "registerToggleMode":
			engine.ToggleRegisterMode()
		case "registerSave":
			engine.SaveRegister()
		case "registerCancel":
			engine.CancelRegister()
		case "setCursor":
			engine.SetCursor(req.Pos)
		case "setSelection":
			engine.SetSelection(req.Anchor, req.Pos)
		case "addDict":
			path := strings.TrimSpace(req.Path)
			if path == "" {
				emit(map[string]any{"type": "error", "message": "追加辞書のパスが空です"})
				continue
			}
			if slices.Contains(extraDicts, path) {
				emitConfig()
				continue
			}
			if !loadDict(path) {
				continue // loadDict already emitted the error
			}
			// loadDict appended to `loaded`; keep the extra list too.
			extraDicts = append(extraDicts, path)
			if st != nil {
				if err := st.Save(store.ExtraDictsFile, marshalStringList(extraDicts)); err != nil {
					log.Printf("save %s: %v", store.ExtraDictsFile, err)
				}
			}
			emitConfig()
			continue
		case "removeDict":
			path := strings.TrimSpace(req.Path)
			next := extraDicts[:0]
			for _, existing := range extraDicts {
				if existing != path {
					next = append(next, existing)
				}
			}
			extraDicts = next
			if st != nil {
				if err := st.Save(store.ExtraDictsFile, marshalStringList(extraDicts)); err != nil {
					log.Printf("save %s: %v", store.ExtraDictsFile, err)
				}
			}
			// Entries already merged in memory stay until the next start.
			emitConfig()
			continue
		case "state":
		case "quit":
			if st != nil {
				st.Flush()
			}
			pasteWG.Wait()
			return nil
		default:
			emit(map[string]any{"type": "error", "message": "unknown op: " + req.Op})
			continue
		}
		state := engine.State()
		if state.Copied && cfg.Clipboard.AutoPaste {
			pasteArmed = true
		}
		emit(stateMessage(state))
	}
	if st != nil {
		st.Flush()
	}
	pasteWG.Wait()
	return scanner.Err()
}

// stateEnvelope tags the render state for the wire. The embedded State's
// fields marshal inline, so the payload is {"text":…,…,"type":"state"}.
type stateEnvelope struct {
	skk.State
	Type string `json:"type"`
}

func stateMessage(s skk.State) stateEnvelope {
	return stateEnvelope{State: s, Type: "state"}
}
