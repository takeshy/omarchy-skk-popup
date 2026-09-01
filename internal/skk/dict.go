package skk

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// Dictionary merges three candidate sources in priority order: the
// learned candidate history, the user dictionary, and the system
// dictionaries (SKK-JISYO.*). History and user entries are persisted as
// JSON through the Persister callbacks.
type Dictionary struct {
	system  map[string][]string
	user    map[string][]string
	history map[string][]string
	cache   map[string][]string
}

func NewDictionary() *Dictionary {
	return &Dictionary{
		system:  map[string][]string{},
		user:    map[string][]string{},
		history: map[string][]string{},
		cache:   map[string][]string{},
	}
}

// LoadSystemFile parses one SKK-JISYO style dictionary (EUC-JP or UTF-8)
// or a JSON map ({"reading": ["candidate", ...]}) into the system table.
// Later files never override earlier candidates for the same word, so
// the load order is the priority order.
func (d *Dictionary) LoadSystemFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		var table map[string][]string
		if err := json.Unmarshal(data, &table); err != nil {
			return err
		}
		keys := make([]string, 0, len(table))
		for k := range table {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			d.addSystem(k, table[k])
		}
		d.cache = map[string][]string{}
		return nil
	}
	return d.ParseSystem(bytes.NewReader(data))
}

// ParseSystem reads SKK-JISYO text. EUC-JP input is transcoded; input
// that is already valid UTF-8 is used as is.
func (d *Dictionary) ParseSystem(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if !utf8.Valid(data) {
		decoded, _, err := transform.Bytes(japanese.EUCJP.NewDecoder(), data)
		if err != nil {
			return err
		}
		data = decoded
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";;") {
			continue
		}
		space := strings.IndexByte(line, ' ')
		if space < 0 {
			continue
		}
		kana := line[:space]
		rest := line[space+1:]
		if !strings.HasPrefix(rest, "/") || !strings.HasSuffix(rest, "/") {
			continue
		}
		d.addSystem(kana, strings.Split(rest[1:len(rest)-1], "/"))
	}
	d.cache = map[string][]string{}
	return scanner.Err()
}

func (d *Dictionary) addSystem(kana string, entries []string) {
	bucket := d.system[kana]
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		word := candidateWord(entry)
		if word == "" || containsWord(bucket, word) {
			continue
		}
		bucket = append(bucket, entry)
	}
	if len(bucket) > 0 {
		d.system[kana] = bucket
	}
}

func containsWord(list []string, word string) bool {
	for _, item := range list {
		if candidateWord(item) == word {
			return true
		}
	}
	return false
}

// SystemEntries reports how many readings the system dictionaries hold.
func (d *Dictionary) SystemEntries() int { return len(d.system) }

// SetUserJSON / SetHistoryJSON install persisted tables. Invalid JSON or
// an unexpected shape is ignored, matching the frontend's tolerance.
func (d *Dictionary) SetUserJSON(raw string) {
	if table, ok := parseCandidateTable(raw); ok {
		d.user = table
	}
	d.cache = map[string][]string{}
}

func (d *Dictionary) SetHistoryJSON(raw string) {
	if table, ok := parseCandidateTable(raw); ok {
		d.history = table
	}
	d.cache = map[string][]string{}
}

func parseCandidateTable(raw string) (map[string][]string, bool) {
	if strings.TrimSpace(raw) == "" {
		return map[string][]string{}, true
	}
	var table map[string][]string
	if err := json.Unmarshal([]byte(raw), &table); err != nil {
		return nil, false
	}
	if table == nil {
		table = map[string][]string{}
	}
	return table, true
}

func (d *Dictionary) UserJSON() string    { return marshalTable(d.user) }
func (d *Dictionary) HistoryJSON() string { return marshalTable(d.history) }

func marshalTable(table map[string][]string) string {
	data, err := json.Marshal(table)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// Lookup merges history, user, and system candidates for one reading,
// deduplicated by word (the part before ';').
func (d *Dictionary) Lookup(kana string) []string {
	if cached, ok := d.cache[kana]; ok {
		return cached
	}
	merged := mergeCandidateLists(d.history[kana], d.user[kana], d.system[kana])
	d.cache[kana] = merged
	return merged
}

func mergeCandidateLists(lists ...[]string) []string {
	merged := []string{}
	seen := map[string]bool{}
	for _, list := range lists {
		for _, candidate := range list {
			word := candidateWord(candidate)
			if word == "" || seen[word] {
				continue
			}
			seen[word] = true
			merged = append(merged, candidate)
		}
	}
	return merged
}

type lookupSpec struct {
	key     string
	numbers []string // non-nil for the numeric (#) form of the reading
}

// lookupSpecs returns the plain reading plus, when it contains digits, the
// "#"-normalised reading with the captured numbers.
func lookupSpecs(primary string) []lookupSpec {
	specs := []lookupSpec{{key: primary}}
	if hasDigitRe.MatchString(primary) {
		specs = append(specs, lookupSpec{
			key:     digitRun.ReplaceAllString(primary, "#"),
			numbers: digitRun.FindAllString(primary, -1),
		})
	}
	return specs
}

func (d *Dictionary) lookupAny(specs []lookupSpec) []string {
	merged := []string{}
	seen := map[string]bool{}
	for _, spec := range specs {
		for _, candidate := range d.Lookup(spec.key) {
			text := candidate
			if spec.numbers != nil {
				text = applyNumericCandidate(candidate, spec.numbers)
			}
			word := candidateWord(text)
			if word == "" || seen[word] {
				continue
			}
			seen[word] = true
			merged = append(merged, text)
		}
	}
	return merged
}

// RememberSelection moves a chosen candidate to the front of the history
// for its reading (keeping at most eight).
func (d *Dictionary) RememberSelection(key, candidate string) bool {
	if key == "" || candidate == "" {
		return false
	}
	next := []string{candidate}
	for _, item := range d.history[key] {
		if item != candidate {
			next = append(next, item)
		}
	}
	if len(next) > 8 {
		next = next[:8]
	}
	d.history[key] = next
	delete(d.cache, key)
	return true
}

// RegisterWord stores a user-entered word for a reading at the front of
// the user dictionary.
func (d *Dictionary) RegisterWord(key, value string) {
	next := []string{value}
	for _, candidate := range d.user[key] {
		if candidateWord(candidate) != value {
			next = append(next, candidate)
		}
	}
	d.user[key] = next
	d.cache = map[string][]string{}
}

// Purge removes a word from both the history and the user dictionary.
func (d *Dictionary) Purge(key, word string) {
	for _, table := range []map[string][]string{d.history, d.user} {
		list, ok := table[key]
		if !ok {
			continue
		}
		kept := list[:0:0]
		for _, item := range list {
			if candidateWord(item) != word {
				kept = append(kept, item)
			}
		}
		if len(kept) == 0 {
			delete(table, key)
		} else {
			table[key] = kept
		}
	}
	d.cache = map[string][]string{}
}

var completionExcludeRe = regexp.MustCompile(`[a-z>#]`)

// CompletionKeys returns readings from the history and user dictionary
// that extend prefix (excluding okuri-ari and numeric keys), sorted.
func (d *Dictionary) CompletionKeys(prefix string) []string {
	seen := map[string]bool{}
	var keys []string
	for _, table := range []map[string][]string{d.history, d.user} {
		for key := range table {
			if seen[key] || key == prefix || !strings.HasPrefix(key, prefix) || completionExcludeRe.MatchString(key) {
				continue
			}
			seen[key] = true
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// ErrNoDictionary is returned by loaders when no system dictionary could
// be found; the engine still works with history/user entries only.
var ErrNoDictionary = errors.New("no system dictionary loaded")
