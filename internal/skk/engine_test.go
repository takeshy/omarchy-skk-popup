package skk

import (
	"encoding/json"
	"strings"
	"testing"
)

const testDict = `;; -*- mode: fundamental; coding: utf-8 -*-
;; okuri-ari entries.
かんj /感/
おくr /送/
;; okuri-nasi entries.
にほんご /日本語/二本語;annotation/
かんじ /漢字/幹事/監事/感じ/寛治/勘司/莞爾/
だい#かい /第#1回/第#3回/
ちょう> /超/
>てき /的/
`

type fakeClip struct {
	copied []string
	text   string
}

func (f *fakeClip) Copy(text string) error { f.copied = append(f.copied, text); return nil }
func (f *fakeClip) Read() (string, error)  { return f.text, nil }

type fakePersister struct{ user, history, input string }

func (p *fakePersister) SaveUserDict(j string)     { p.user = j }
func (p *fakePersister) SaveHistory(j string)      { p.history = j }
func (p *fakePersister) SaveInputHistory(j string) { p.input = j }

func newTestEngine(t *testing.T) (*Engine, *fakeClip, *fakePersister) {
	t.Helper()
	dict := NewDictionary()
	if err := dict.ParseSystem(strings.NewReader(testDict)); err != nil {
		t.Fatal(err)
	}
	clip := &fakeClip{}
	persister := &fakePersister{}
	return New(dict, clip, persister), clip, persister
}

func typeKeys(e *Engine, keys string) {
	for _, r := range keys {
		e.HandleKey(Key{Key: string(r)})
	}
}

func press(e *Engine, name string) { e.HandleKey(Key{Key: name}) }

func TestRomajiToKana(t *testing.T) {
	e, _, _ := newTestEngine(t)
	typeKeys(e, "nihongo")
	if got := e.Text(); got != "にほんご" {
		t.Fatalf("text = %q", got)
	}
	typeKeys(e, "kk")
	if s := e.State(); s.Text != "にほんごっk" {
		t.Fatalf("pending roman not shown: %q", s.Text)
	}
	typeKeys(e, "a")
	if got := e.Text(); got != "にほんごっか" {
		t.Fatalf("text = %q", got)
	}
	typeKeys(e, "n")
	press(e, "Enter") // flushes the pending n
	if got := e.Text(); got != "にほんごっかん" {
		t.Fatalf("text = %q", got)
	}
}

func TestConversionAndCommit(t *testing.T) {
	e, _, p := newTestEngine(t)
	typeKeys(e, "Nihongo")
	s := e.State()
	if s.Text != "▽にほんご" || s.Mode != "SKK 変換" {
		t.Fatalf("preedit state = %+v", s)
	}
	press(e, " ")
	s = e.State()
	if s.Text != "日本語" || !s.CandidateActive || s.Candidate != "日本語" || s.Mode != "SKK 候補" {
		t.Fatalf("candidate state = %+v", s)
	}
	press(e, " ")
	s = e.State()
	if s.Candidate != "二本語 ※annotation" {
		t.Fatalf("annotation not shown: %q", s.Candidate)
	}
	press(e, "x")
	if s = e.State(); s.Candidate != "日本語" {
		t.Fatalf("x should go back: %q", s.Candidate)
	}
	press(e, "Enter")
	if got := e.Text(); got != "日本語" {
		t.Fatalf("text = %q", got)
	}
	if !strings.Contains(p.history, `"にほんご":["日本語"]`) {
		t.Fatalf("history not persisted: %s", p.history)
	}
	// The learned candidate is now first, even for a different order.
	e2, _, _ := newTestEngine(t)
	e2.dict.SetHistoryJSON(`{"にほんご":["二本語"]}`)
	typeKeys(e2, "Nihongo ")
	// History stores the bare word, so the annotation is gone once learned.
	if s := e2.State(); s.Candidate != "二本語" {
		t.Fatalf("history should be first: %q", s.Candidate)
	}
}

func TestOkuriAutoConvert(t *testing.T) {
	e, _, _ := newTestEngine(t)
	typeKeys(e, "KanJi")
	s := e.State()
	if s.Text != "感じ" || !s.CandidateActive {
		t.Fatalf("okuri state = %+v", s)
	}
	press(e, "Enter")
	if got := e.Text(); got != "感じ" {
		t.Fatalf("text = %q", got)
	}
}

func TestStickyShift(t *testing.T) {
	e, _, _ := newTestEngine(t)
	typeKeys(e, ";okuri")
	if s := e.State(); s.Text != "▽おくり" {
		t.Fatalf("sticky start: %q", s.Text)
	}
	e.main.resetComposition()
	typeKeys(e, ";oku;ru")
	s := e.State()
	if s.Text != "送る" {
		t.Fatalf("sticky okuri: %+v", s)
	}
}

func TestCandidateList(t *testing.T) {
	e, _, _ := newTestEngine(t)
	typeKeys(e, "Kanji")
	for i := 0; i < 5; i++ {
		press(e, " ")
	}
	s := e.State()
	if !strings.HasPrefix(s.Candidate, "A:寛治  S:勘司  D:莞爾  [5-7/7]") {
		t.Fatalf("list = %q", s.Candidate)
	}
	press(e, "s")
	if got := e.Text(); got != "勘司" {
		t.Fatalf("label select = %q", got)
	}
}

func TestNumericConversion(t *testing.T) {
	e, _, _ := newTestEngine(t)
	typeKeys(e, "Dai5kai ")
	if s := e.State(); s.Candidate != "第５回" {
		t.Fatalf("numeric = %q", s.Candidate)
	}
	press(e, " ")
	if s := e.State(); s.Candidate != "第五回" {
		t.Fatalf("numeric kanji = %q", s.Candidate)
	}
	if got := toKanjiNumeral("1234"); got != "千二百三十四" {
		t.Fatalf("kanji numeral = %q", got)
	}
	if got := toKanjiNumeral("10000"); got != "一万" {
		t.Fatalf("kanji numeral = %q", got)
	}
	if got := toKanjiNumeral("0"); got != "〇" {
		t.Fatalf("kanji numeral = %q", got)
	}
}

func TestPrefixSuffix(t *testing.T) {
	e, _, _ := newTestEngine(t)
	typeKeys(e, "Chou>")
	if s := e.State(); s.Candidate != "超" {
		t.Fatalf("prefix = %+v", s)
	}
	press(e, "Enter")
	typeKeys(e, "Nihongo ")
	press(e, "Enter")
	typeKeys(e, ">teki ")
	press(e, "Enter")
	if got := e.Text(); got != "超日本語的" {
		t.Fatalf("text = %q", got)
	}
}

func TestRegisterFlow(t *testing.T) {
	e, _, p := newTestEngine(t)
	typeKeys(e, "Mitei ")
	s := e.State()
	if !s.Register.Open || s.Register.Reading != "みてい" {
		t.Fatalf("register should open: %+v", s.Register)
	}
	typeKeys(e, "Mi")
	press(e, " ") // no candidates inside the dialog either
	if s = e.State(); s.Register.Error == "" {
		t.Fatalf("expected an error for missing candidates")
	}
	press(e, "Escape") // cancel composition is not offered; Escape closes dialog
	if e.RegisterOpen() {
		t.Fatalf("escape should close the dialog")
	}
	if s = e.State(); s.Text != "▽みてい" {
		t.Fatalf("main preedit should survive: %q", s.Text)
	}
	press(e, " ")
	// Type a katakana word via q, then commit with Enter.
	typeKeys(e, "l") // ascii mode inside the dialog
	typeKeys(e, "TBD")
	press(e, "Enter")
	if e.RegisterOpen() {
		t.Fatalf("Enter should save")
	}
	if got := e.Text(); got != "TBD" {
		t.Fatalf("text = %q", got)
	}
	if !strings.Contains(p.user, `"みてい":["TBD"]`) {
		t.Fatalf("user dict not persisted: %s", p.user)
	}
	if s = e.State(); s.Status != "Registered." {
		t.Fatalf("status = %q", s.Status)
	}
	// The registered word converts immediately next time.
	typeKeys(e, "Mitei ")
	if s = e.State(); s.Candidate != "TBD" {
		t.Fatalf("registered word missing: %+v", s)
	}
}

func TestCopyAndClose(t *testing.T) {
	e, clip, p := newTestEngine(t)
	typeKeys(e, "Nihongo ")
	press(e, "Enter") // commit the candidate
	press(e, "Enter") // copy
	s := e.State()
	if !s.Close || !s.Copied || s.Text != "" {
		t.Fatalf("copy state = %+v", s)
	}
	if len(clip.copied) != 1 || clip.copied[0] != "日本語" {
		t.Fatalf("clipboard = %v", clip.copied)
	}
	if p.input != `["日本語"]` {
		t.Fatalf("input history = %s", p.input)
	}
	// Up recalls the history, Down returns to the draft.
	typeKeys(e, "a")
	press(e, "Up")
	if s = e.State(); s.Text != "日本語" {
		t.Fatalf("history recall = %q", s.Text)
	}
	press(e, "Down")
	if s = e.State(); s.Text != "あ" {
		t.Fatalf("draft restore = %q", s.Text)
	}
	// Enter on empty text does not close.
	press(e, "Backspace")
	press(e, "Enter")
	if s = e.State(); s.Close || s.Status != "Nothing to copy." {
		t.Fatalf("empty copy = %+v", s)
	}
}

func TestEscapeClosesWhenIdle(t *testing.T) {
	e, _, _ := newTestEngine(t)
	typeKeys(e, "Nihongo")
	press(e, "Escape")
	if s := e.State(); s.Close || s.Text != "" {
		t.Fatalf("escape should only cancel composition: %+v", s)
	}
	typeKeys(e, "a")
	press(e, "Escape")
	if s := e.State(); !s.Close || s.Text != "あ" {
		t.Fatalf("escape should close and keep text: %+v", s)
	}
}

func TestModes(t *testing.T) {
	e, _, _ := newTestEngine(t)
	typeKeys(e, "q")
	if s := e.State(); s.Mode != "SKK カナ" {
		t.Fatalf("mode = %q", s.Mode)
	}
	typeKeys(e, "ka")
	if got := e.Text(); got != "カ" {
		t.Fatalf("katakana = %q", got)
	}
	e.HandleKey(Key{Key: "q", Ctrl: true})
	typeKeys(e, "ka")
	if got := e.Text(); got != "カｶ" {
		t.Fatalf("half katakana = %q", got)
	}
	e.HandleKey(Key{Key: "j", Ctrl: true})
	typeKeys(e, "l")
	typeKeys(e, "abc ")
	if got := e.Text(); got != "カｶabc " {
		t.Fatalf("ascii = %q", got)
	}
	e.HandleKey(Key{Key: "j", Ctrl: true})
	typeKeys(e, "L")
	typeKeys(e, "a1")
	if got := e.Text(); got != "カｶabc ａ１" {
		t.Fatalf("wide ascii = %q", got)
	}
	e.ToggleMode()
	if s := e.State(); s.Mode != "SKK かな" {
		t.Fatalf("toggle back = %q", s.Mode)
	}
	typeKeys(e, "Kanji")
	typeKeys(e, "q")
	if got := e.Text(); !strings.HasSuffix(got, "カンジ") {
		t.Fatalf("q katakana commit = %q", got)
	}
}

func TestZCommandsAndAbbrev(t *testing.T) {
	e, _, _ := newTestEngine(t)
	typeKeys(e, "zh")
	typeKeys(e, "z ")
	if got := e.Text(); got != "←　" {
		t.Fatalf("z commands = %q", got)
	}
	typeKeys(e, "/abc")
	if s := e.State(); s.Text != "←　▽/abc" || s.Mode != "SKK 略語" {
		t.Fatalf("abbrev = %+v", s)
	}
	press(e, "Backspace")
	press(e, "Backspace")
	press(e, "Backspace")
	press(e, "Escape")
	if s := e.State(); s.Text != "←　" || s.Mode != "SKK かな" {
		t.Fatalf("abbrev cancel = %+v", s)
	}
	typeKeys(e, "//")
	if got := e.Text(); got != "←　/" {
		t.Fatalf("literal slash = %q", got)
	}
}

func TestBackspaceAndCaret(t *testing.T) {
	e, _, _ := newTestEngine(t)
	typeKeys(e, "aiu")
	press(e, "Left")
	typeKeys(e, "e")
	if got := e.Text(); got != "あいえう" {
		t.Fatalf("insert at caret = %q", got)
	}
	press(e, "Backspace")
	press(e, "Home")
	press(e, "Delete")
	if got := e.Text(); got != "いう" {
		t.Fatalf("delete = %q", got)
	}
	e.SetCursor(2)
	typeKeys(e, "Ni")
	press(e, "Backspace") // empties the reading and leaves composition
	if s := e.State(); s.Text != "いう" || s.Mode != "SKK かな" {
		t.Fatalf("backspace out of composition = %+v", s)
	}
	e.HandleKey(Key{Key: "Enter", Shift: true})
	if got := e.Text(); got != "いう\n" {
		t.Fatalf("newline = %q", got)
	}
}

func shiftPress(e *Engine, name string) { e.HandleKey(Key{Key: name, Shift: true}) }

func TestShiftSelection(t *testing.T) {
	e, clip, _ := newTestEngine(t)
	typeKeys(e, "aiueo") // あいうえお, cursor 5
	press(e, "Home")     // cursor 0
	shiftPress(e, "Right")
	shiftPress(e, "Right") // select あい
	if s := e.State(); s.SelStart != 0 || s.SelEnd != 2 || s.Cursor != 2 {
		t.Fatalf("selection = %+v", s)
	}
	// Ctrl+C copies just the selection, without closing.
	e.HandleKey(Key{Key: "c", Ctrl: true})
	if len(clip.copied) != 1 || clip.copied[0] != "あい" {
		t.Fatalf("ctrl+c = %v", clip.copied)
	}
	// Typing over the selection replaces it.
	typeKeys(e, "ka")
	if got := e.Text(); got != "かうえお" {
		t.Fatalf("replace-selection = %q", got)
	}
	// Ctrl+O select-all + Ctrl+X cut clears the buffer to the clipboard.
	e.HandleKey(Key{Key: "o", Ctrl: true})
	if s := e.State(); s.SelStart != 0 || s.SelEnd != 4 {
		t.Fatalf("select all = %+v", s)
	}
	e.HandleKey(Key{Key: "x", Ctrl: true})
	if got := e.Text(); got != "" || clip.copied[len(clip.copied)-1] != "かうえお" {
		t.Fatalf("cut = %q clip=%v", got, clip.copied)
	}
}

func TestEmacsBindings(t *testing.T) {
	e, _, _ := newTestEngine(t)
	typeKeys(e, "aiueo") // あいうえお
	e.HandleKey(Key{Key: "h", Ctrl: true})
	if s := e.State(); s.Cursor != 0 {
		t.Fatalf("C-h (head) cursor = %d", s.Cursor)
	}
	e.HandleKey(Key{Key: "a", Ctrl: true}) // select all
	if s := e.State(); s.SelStart != 0 || s.SelEnd != 5 {
		t.Fatalf("C-a select all = %+v", s)
	}
	e.HandleKey(Key{Key: "h", Ctrl: true}) // collapse to head (no shift -> clears sel)
	e.HandleKey(Key{Key: "f", Ctrl: true})
	e.HandleKey(Key{Key: "f", Ctrl: true}) // cursor 2
	e.HandleKey(Key{Key: "k", Ctrl: true}) // kill to EOL -> あい
	if got := e.Text(); got != "あい" {
		t.Fatalf("C-k = %q", got)
	}
	e.HandleKey(Key{Key: "e", Ctrl: true}) // EOL (already)
	typeKeys(e, "u")                       // あいう
	e.HandleKey(Key{Key: "b", Ctrl: true}) // cursor 2
	e.HandleKey(Key{Key: "u", Ctrl: true}) // kill to line start -> う
	if got := e.Text(); got != "う" {
		t.Fatalf("C-u = %q", got)
	}
	// Ctrl+Z undoes the last kill.
	e.HandleKey(Key{Key: "z", Ctrl: true})
	if got := e.Text(); got != "あいう" {
		t.Fatalf("C-z = %q", got)
	}
	e.HandleKey(Key{Key: "z", Ctrl: true}) // undo the 'u' insert
	e.HandleKey(Key{Key: "z", Ctrl: true}) // undo the C-k
	if got := e.Text(); got != "あいうえお" {
		t.Fatalf("C-z chain = %q", got)
	}
}

func TestVerticalCaretVsHistory(t *testing.T) {
	e, clip, _ := newTestEngine(t)
	// Seed a history entry.
	typeKeys(e, "Nihongo ")
	press(e, "Enter")
	press(e, "Enter") // copy "日本語" -> history, buffer cleared
	if clip.copied[0] != "日本語" {
		t.Fatalf("setup copy = %v", clip.copied)
	}
	// Multi-line draft.
	typeKeys(e, "a")
	e.HandleKey(Key{Key: "Enter", Shift: true})
	typeKeys(e, "i")
	e.HandleKey(Key{Key: "Enter", Shift: true})
	typeKeys(e, "u") // "あ\nい\nう", cursor at end (line 2)
	if e.Text() != "あ\nい\nう" {
		t.Fatalf("draft = %q", e.Text())
	}
	// Up from the last line moves to the previous line, not history.
	press(e, "Up")
	if s := e.State(); s.Text != "あ\nい\nう" || s.Cursor != 3 {
		t.Fatalf("up to line 1 = cursor %d text %q", s.Cursor, s.Text)
	}
	press(e, "Up") // now on line 0
	if s := e.State(); s.Cursor != 1 || s.Text != "あ\nい\nう" {
		t.Fatalf("up to line 0 = cursor %d", s.Cursor)
	}
	// Up again from line 0 -> history recall.
	press(e, "Up")
	if s := e.State(); s.Text != "日本語" {
		t.Fatalf("up from line 0 should recall history: %q", s.Text)
	}
	// Down returns to the draft (last line boundary).
	press(e, "Down")
	if s := e.State(); s.Text != "あ\nい\nう" {
		t.Fatalf("down restores draft: %q", s.Text)
	}
}

func TestCompletionAndPurge(t *testing.T) {
	e, _, p := newTestEngine(t)
	e.dict.SetUserJSON(`{"かんじょう":["感情"]}`)
	typeKeys(e, "Kann")
	press(e, "Tab")
	if s := e.State(); s.Text != "▽かんじょう" {
		t.Fatalf("completion = %q", s.Text)
	}
	press(e, " ")
	if s := e.State(); s.Candidate != "感情" {
		t.Fatalf("completed lookup = %+v", s)
	}
	press(e, "X")
	if s := e.State(); s.CandidateActive || !strings.Contains(p.user, "{}") {
		t.Fatalf("purge = %+v user=%s", s, p.user)
	}
}

func TestPasteAndExternalClipboard(t *testing.T) {
	e, clip, _ := newTestEngine(t)
	clip.text = "外部\r\nテキスト"
	e.CaptureExternalClipboard()
	press(e, "Up")
	if s := e.State(); s.Text != "外部\r\nテキスト" {
		t.Fatalf("external capture = %q", s.Text)
	}
	press(e, "Down")
	typeKeys(e, "Ka")
	e.HandleKey(Key{Key: "v", Ctrl: true})
	if got := e.Text(); got != "か外部\nテキスト" {
		t.Fatalf("paste = %q", got)
	}
}

func TestStateJSONShape(t *testing.T) {
	e, _, _ := newTestEngine(t)
	data, err := json.Marshal(e.State())
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"text"`, `"cursor"`, `"mode"`, `"candidate"`, `"status"`, `"register"`, `"close"`} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("missing %s in %s", field, data)
		}
	}
}

func TestEUCJPDictionary(t *testing.T) {
	// "あ /亜/" in EUC-JP.
	raw := []byte{0xa4, 0xa2, ' ', '/', 0xb0, 0xa1, '/', '\n'}
	dict := NewDictionary()
	if err := dict.ParseSystem(strings.NewReader(string(raw))); err != nil {
		t.Fatal(err)
	}
	if got := dict.Lookup("あ"); len(got) != 1 || got[0] != "亜" {
		t.Fatalf("lookup = %v", got)
	}
}
