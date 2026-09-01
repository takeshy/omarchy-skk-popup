package skk

import "strings"

const (
	defaultStatus     = "Enter: copy / Shift+Enter: newline / Space: convert"
	candidateStatus   = "Space: next / Enter: commit / x: previous"
	inlineCandidates  = 4
	listPageSize      = 7
	inputHistoryLimit = 30
)

var listLabels = []string{"a", "s", "d", "f", "j", "k", "l"}

var zCommands = map[string]string{
	"h": "←", "j": "↓", "k": "↑", "l": "→",
	" ": "　", ".": "…", ",": "‥", "-": "～", "/": "・", "[": "『", "]": "』",
}

// Key is one key press as delivered by the UI. Key is either a single
// printable ASCII character (" " for Space) or one of the named keys
// "Enter", "Backspace", "Escape", "Tab", "Up", "Down", "Left", "Right",
// "Home", "End", "Delete".
type Key struct {
	Key   string `json:"key"`
	Ctrl  bool   `json:"ctrl"`
	Shift bool   `json:"shift"`
	Alt   bool   `json:"alt"`
}

// Clipboard is the system clipboard the engine copies to and reads from.
type Clipboard interface {
	Copy(text string) error
	Read() (string, error)
}

// Persister receives the JSON documents the engine wants stored. Every
// method may be nil-safe: a nil Persister disables persistence.
type Persister interface {
	SaveUserDict(json string)
	SaveHistory(json string)
	SaveInputHistory(json string)
}

// Engine is the SKK popup state machine: the clipboard input buffer, the
// word-registration dialog, and the input history, driven by HandleKey.
type Engine struct {
	dict      *Dictionary
	clip      Clipboard
	persister Persister

	main composer
	reg  composer

	registerOpen  bool
	registerKey   string
	registerError string

	status string

	inputHistory      []string
	inputHistoryIndex int
	inputHistoryDraft []rune

	// closeRequested is set for one HandleKey/Do call when the UI should
	// hide the popup; copied reports that text was placed on the clipboard
	// during that call (so the UI can trigger auto paste after hiding).
	closeRequested bool
	copied         bool
}

func New(dict *Dictionary, clip Clipboard, persister Persister) *Engine {
	if dict == nil {
		dict = NewDictionary()
	}
	return &Engine{
		dict:              dict,
		clip:              clip,
		persister:         persister,
		status:            defaultStatus,
		inputHistoryIndex: -1,
	}
}

// SetInputHistoryJSON installs the persisted clipboard input history.
func (e *Engine) SetInputHistoryJSON(raw string) {
	e.inputHistory = parseInputHistory(raw)
}

// ---- persistence helpers ---------------------------------------------------

func (e *Engine) persistUserDict() {
	if e.persister != nil {
		e.persister.SaveUserDict(e.dict.UserJSON())
	}
}

func (e *Engine) persistHistory() {
	if e.persister != nil {
		e.persister.SaveHistory(e.dict.HistoryJSON())
	}
}

func (e *Engine) persistInputHistory() {
	if e.persister != nil {
		e.persister.SaveInputHistory(marshalStrings(e.inputHistory))
	}
}

func (e *Engine) addInputHistory(text string) {
	if text == "" {
		return
	}
	next := make([]string, 0, len(e.inputHistory)+1)
	for _, entry := range e.inputHistory {
		if entry != text {
			next = append(next, entry)
		}
	}
	next = append(next, text)
	if len(next) > inputHistoryLimit {
		next = next[len(next)-inputHistoryLimit:]
	}
	e.inputHistory = next
	e.inputHistoryIndex = -1
	e.inputHistoryDraft = nil
	e.persistInputHistory()
}

// Shown is called when the popup becomes visible: a stale "Copied." status
// goes back to the default hint and the external clipboard is captured.
func (e *Engine) Shown() {
	if e.status == "Copied." || e.status == "Nothing to copy." || e.status == "Copy failed." {
		e.status = defaultStatus
	}
	e.CaptureExternalClipboard()
}

// CaptureExternalClipboard appends text copied by other applications to
// the input history so ↑ can recall it. Called when the popup is shown.
func (e *Engine) CaptureExternalClipboard() {
	if e.clip == nil {
		return
	}
	text, err := e.clip.Read()
	if err != nil || text == "" {
		return
	}
	if len(e.inputHistory) > 0 && e.inputHistory[len(e.inputHistory)-1] == text {
		return
	}
	e.addInputHistory(text)
}

func (e *Engine) showInputHistory(direction int) bool {
	if len(e.inputHistory) == 0 {
		return false
	}
	c := &e.main
	if e.inputHistoryIndex == -1 {
		if direction > 0 {
			return false
		}
		e.inputHistoryDraft = append([]rune(nil), c.text...)
		e.inputHistoryIndex = len(e.inputHistory) - 1
	} else {
		e.inputHistoryIndex += direction
		if e.inputHistoryIndex >= len(e.inputHistory) {
			e.inputHistoryIndex = -1
			c.text = append([]rune(nil), e.inputHistoryDraft...)
		} else {
			e.inputHistoryIndex = max(0, e.inputHistoryIndex)
		}
	}
	if e.inputHistoryIndex != -1 {
		c.text = []rune(e.inputHistory[e.inputHistoryIndex])
	}
	c.cursor = len(c.text)
	return true
}

// ---- main buffer: modes ----------------------------------------------------

func (e *Engine) applyKatakanaMode(text string) string {
	c := &e.main
	if c.katakana == "" || c.composing || c.abbrevMode {
		return text
	}
	katakana := toKatakana(text)
	if c.katakana == "han" {
		return toHalfWidthKatakana(katakana)
	}
	return katakana
}

func (e *Engine) candidateText() string {
	c := &e.main
	raw := c.currentCandidate()
	stem := c.kana
	if raw != "" {
		stem = candidateWord(raw)
	}
	if c.okuriKey != "" {
		return stem + c.okuriKana
	}
	return stem
}

func (e *Engine) candidateListActive() bool {
	c := &e.main
	return c.composing && c.showingCandidate && c.candidateIndex >= inlineCandidates
}

func (e *Engine) candidateListText() string {
	c := &e.main
	start := c.candidateIndex
	end := min(start+listPageSize, len(c.candidates))
	parts := make([]string, 0, listPageSize+1)
	for i := start; i < end; i++ {
		parts = append(parts, strings.ToUpper(listLabels[i-start])+":"+candidateWord(c.candidates[i]))
	}
	parts = append(parts, "["+itoa(start+1)+"-"+itoa(end)+"/"+itoa(len(c.candidates))+"]")
	return strings.Join(parts, "  ")
}

func (e *Engine) currentPreeditText() string {
	c := &e.main
	if c.abbrevMode {
		return c.abbrevPreedit()
	}
	if !c.composing {
		return c.roman
	}
	if c.showingCandidate {
		return e.candidateText()
	}
	return c.composingPreedit() + c.roman
}

func (e *Engine) enterKanaMode() {
	c := &e.main
	c.asciiMode = false
	c.wideAscii = false
	c.katakana = ""
}

func (e *Engine) commitPendingForModeSwitch() {
	c := &e.main
	if c.showingCandidate {
		e.commitCandidate()
	} else if c.composing || c.roman != "" {
		if !e.flushPendingRoman() && c.roman != "" {
			c.insertText(c.roman)
			c.roman = ""
		}
		if c.composing {
			e.commitRawPreedit()
		}
	}
	c.resetComposition()
}

func (e *Engine) enterAsciiMode() {
	e.commitPendingForModeSwitch()
	e.main.asciiMode = true
	e.main.wideAscii = false
}

func (e *Engine) enterWideAsciiMode() {
	e.commitPendingForModeSwitch()
	e.main.asciiMode = false
	e.main.wideAscii = true
}

// ToggleMode flips between kana input and ASCII (the mode badge click).
func (e *Engine) ToggleMode() {
	if e.main.asciiMode || e.main.wideAscii {
		e.enterKanaMode()
	} else {
		e.enterAsciiMode()
	}
}

// ---- main buffer: composition ---------------------------------------------

func (e *Engine) commitCandidate() {
	c := &e.main
	if !c.composing {
		return
	}
	committedKey := c.lookupKey()
	selected := ""
	if c.showingCandidate {
		selected = candidateWord(c.currentCandidate())
	}
	text := c.preeditKana()
	if c.showingCandidate {
		text = e.candidateText()
	}
	c.insertText(text)
	if selected != "" && e.dict.RememberSelection(committedKey, selected) {
		e.persistHistory()
	}
	c.resetComposition()
}

func (e *Engine) commitRawPreedit() {
	c := &e.main
	if !c.composing {
		return
	}
	c.insertText(c.preeditKana())
	c.resetComposition()
}

func (e *Engine) commitKatakana(half bool) bool {
	c := &e.main
	if !c.composing || c.preeditKana() == "" {
		return false
	}
	katakana := toKatakana(c.preeditKana())
	if half {
		katakana = toHalfWidthKatakana(katakana)
	}
	c.insertText(katakana)
	c.resetComposition()
	return true
}

func (e *Engine) toggleKatakanaMode(kind string) {
	if e.main.katakana == kind {
		e.main.katakana = ""
	} else {
		e.main.katakana = kind
	}
}

func (e *Engine) showPreedit()   { e.main.showingCandidate = false }
func (e *Engine) showCandidate() { e.main.showingCandidate = true }

func (e *Engine) startAbbrev() {
	c := &e.main
	c.resetComposition()
	c.abbrevMode = true
	c.abbrev = ""
}

func (e *Engine) closeAbbrev(replacement string) {
	e.main.insertText(replacement)
	e.main.resetComposition()
}

func (e *Engine) startComposition() {
	c := &e.main
	c.composing = true
	c.kana = ""
	c.okuriKey = ""
	c.okuriKana = ""
	c.candidates = nil
	c.candidateIndex = 0
	c.showingCandidate = false
}

func (e *Engine) startOkuri(key string) {
	c := &e.main
	if c.roman == "n" {
		c.consumePendingN()
	}
	c.okuriKey = toLowerASCII(key)
	c.okuriKana = ""
	c.stickyOkuri = false
	c.candidates = nil
	c.candidateIndex = 0
	c.showingCandidate = false
}

func (e *Engine) convertRomanChunk() bool {
	c := &e.main
	kana := c.consumeRomanChunk()
	if kana == "" {
		return false
	}
	if !c.composing {
		c.insertText(e.applyKatakanaMode(kana))
	}
	if c.composing && c.okuriKey != "" && c.okuriKana != "" && c.roman == "" && len(c.candidates) == 0 {
		e.autoConvertOkuri()
	}
	return true
}

func (e *Engine) flushPendingRoman() bool {
	c := &e.main
	if c.roman == "" {
		return true
	}
	for guard := 0; c.roman != "" && guard < 8; guard++ {
		before := c.roman
		if e.convertRomanChunk() {
			continue
		}
		if c.roman != before {
			continue
		}
		if c.roman == "n" {
			kana := c.consumePendingN()
			if !c.composing {
				c.insertText(e.applyKatakanaMode(kana))
			}
			return true
		}
		break
	}
	return c.roman == ""
}

func (e *Engine) autoConvertOkuri() {
	c := &e.main
	if !c.composing || c.okuriKey == "" || c.okuriKana == "" || c.roman != "" || len(c.candidates) > 0 {
		return
	}
	candidates := e.dict.lookupAny(lookupSpecs(c.lookupKey()))
	if len(candidates) == 0 {
		e.showPreedit()
		e.openRegisterModal()
		return
	}
	c.candidates = candidates
	c.candidateIndex = 0
	e.showCandidate()
}

func (e *Engine) showNextCandidate() {
	c := &e.main
	if !c.composing || !e.flushPendingRoman() {
		return
	}
	if len(c.candidates) == 0 {
		c.candidates = e.dict.lookupAny(lookupSpecs(c.lookupKey()))
		c.candidateIndex = 0
		if len(c.candidates) == 0 {
			e.showPreedit()
			e.openRegisterModal()
			return
		}
		e.status = candidateStatus
		e.showCandidate()
		return
	}
	if !c.showingCandidate {
		c.candidateIndex = 0
		e.showCandidate()
		return
	}
	step := 1
	if e.candidateListActive() {
		step = listPageSize
	}
	next := c.candidateIndex + step
	exhausted := c.candidateIndex >= len(c.candidates)-1
	if e.candidateListActive() {
		exhausted = next >= len(c.candidates)
	}
	if exhausted {
		e.openRegisterModal()
		return
	}
	c.candidateIndex = next
	e.showCandidate()
}

func (e *Engine) showAbbrevCandidates() {
	c := &e.main
	if !c.abbrevMode || c.abbrev == "" {
		return
	}
	key := c.abbrev
	c.candidates = e.dict.Lookup(key)
	c.candidateIndex = 0
	c.kana = key
	c.okuriKey = ""
	c.okuriKana = ""
	c.roman = ""
	c.abbrev = ""
	c.abbrevMode = false
	c.composing = true
	if len(c.candidates) == 0 {
		e.showPreedit()
		e.openRegisterModal()
		return
	}
	e.status = candidateStatus
	e.showCandidate()
}

func (e *Engine) showPreviousCandidate() bool {
	c := &e.main
	if !c.composing || !c.showingCandidate || len(c.candidates) == 0 {
		return false
	}
	if c.candidateIndex <= 0 {
		e.showPreedit()
		return true
	}
	if e.candidateListActive() {
		if c.candidateIndex-listPageSize >= inlineCandidates {
			c.candidateIndex -= listPageSize
		} else {
			c.candidateIndex = inlineCandidates - 1
		}
		e.showCandidate()
		return true
	}
	c.candidateIndex--
	e.showCandidate()
	return true
}

func (e *Engine) handleCompletion() bool {
	c := &e.main
	if !c.composing || c.showingCandidate || c.okuriKey != "" || c.abbrevMode {
		return false
	}
	current := c.kana
	if c.completionMatches == nil || !containsString(c.completionMatches, current) {
		if current == "" {
			return false
		}
		keys := e.dict.CompletionKeys(current)
		if len(keys) == 0 {
			return false
		}
		c.completionMatches = append([]string{current}, keys...)
		c.completionIndex = 0
	}
	c.completionIndex = (c.completionIndex + 1) % len(c.completionMatches)
	c.kana = c.completionMatches[c.completionIndex]
	c.candidates = nil
	c.candidateIndex = 0
	e.showPreedit()
	return true
}

func (e *Engine) purgeCurrentCandidate() {
	c := &e.main
	if !c.composing || !c.showingCandidate || len(c.candidates) == 0 {
		return
	}
	key := c.lookupKey()
	word := candidateWord(c.currentCandidate())
	if key == "" || word == "" {
		return
	}
	e.dict.Purge(key, word)
	e.persistHistory()
	e.persistUserDict()

	kept := c.candidates[:0:0]
	for _, item := range c.candidates {
		if candidateWord(item) != word {
			kept = append(kept, item)
		}
	}
	c.candidates = kept
	if len(c.candidates) == 0 {
		c.candidateIndex = 0
		e.showPreedit()
		return
	}
	if c.candidateIndex >= len(c.candidates) {
		c.candidateIndex = len(c.candidates) - 1
	}
	e.showCandidate()
}

func (e *Engine) cancelCandidateSelection() bool {
	c := &e.main
	if !c.composing {
		return false
	}
	e.showPreedit()
	c.candidates = nil
	c.candidateIndex = 0
	return true
}

// ---- main buffer: key handlers --------------------------------------------

func (e *Engine) handlePrintable(k Key) bool {
	c := &e.main
	ch := k.Key
	if !isHandledPrintableKey(ch) {
		return false
	}
	if c.showingCandidate {
		e.commitCandidate()
	}
	if isUpperASCII(ch) {
		if !c.composing {
			e.startComposition()
		} else if c.shouldStartOkuri(ch) {
			e.startOkuri(ch)
		}
	} else if c.stickyOkuri && c.composing && c.okuriKey == "" && c.kana != "" && isLowerASCII(ch) {
		e.startOkuri(ch)
	}

	if isDigit(ch) && !c.composing {
		c.insertText(ch)
		return true
	}
	if ch == "?" && !c.composing {
		c.insertText(ch)
		return true
	}
	if isDigit(ch) && c.composing {
		if !e.flushPendingRoman() {
			c.roman = ""
		}
		c.appendComposingKana(ch)
		return true
	}

	c.roman += toLowerASCII(ch)
	for guard := 0; c.roman != "" && guard < 4; guard++ {
		before := c.roman
		if e.convertRomanChunk() {
			continue
		}
		if c.roman != before {
			continue
		}
		break
	}
	return true
}

func (e *Engine) handleLiteralASCII(k Key) bool {
	c := &e.main
	if c.composing || c.abbrevMode || !isASCIIPrintable(k.Key) {
		return false
	}
	c.insertText(k.Key)
	return true
}

func (e *Engine) handleAbbrevPrintable(k Key) bool {
	c := &e.main
	if !c.abbrevMode {
		return false
	}
	if k.Key == "/" {
		e.closeAbbrev("/")
		return true
	}
	if !isAbbrevChar(k.Key) {
		return false
	}
	c.abbrev += k.Key
	return true
}

func (e *Engine) handlePrefixSuffixConversion(k Key) bool {
	c := &e.main
	if k.Key != ">" {
		return false
	}
	if c.composing {
		if c.showingCandidate {
			e.commitCandidate()
			e.startComposition()
			c.appendComposingKana(">")
			return true
		}
		if !e.flushPendingRoman() {
			return true
		}
		c.appendComposingKana(">")
		e.showNextCandidate()
		return true
	}
	if !e.flushPendingRoman() {
		c.roman = ""
	}
	e.startComposition()
	c.appendComposingKana(">")
	return true
}

func (e *Engine) handleZCommand(k Key) bool {
	c := &e.main
	if c.roman != "z" {
		return false
	}
	text, ok := zCommands[k.Key]
	if !ok {
		return false
	}
	if c.showingCandidate {
		e.showPreedit()
		c.candidates = nil
		c.candidateIndex = 0
	}
	c.roman = ""
	if c.composing {
		c.appendComposingKana(text)
	} else {
		c.insertText(text)
	}
	return true
}

func (e *Engine) handleBackspace() {
	c := &e.main
	if c.abbrevMode {
		if c.abbrev != "" {
			c.abbrev = c.abbrev[:len(c.abbrev)-1]
			return
		}
		e.closeAbbrev("")
		return
	}
	if c.roman == "" && !c.composing && c.cursor == 0 {
		return
	}
	if c.roman != "" {
		c.roman = c.roman[:len(c.roman)-1]
		return
	}
	if c.showingCandidate {
		e.showPreedit()
		return
	}
	if c.composing {
		switch {
		case c.okuriKana != "":
			c.okuriKana = trimLastRune(c.okuriKana)
		case c.okuriKey != "":
			c.okuriKey = ""
		case c.kana != "":
			c.kana = trimLastRune(c.kana)
		}
		c.candidates = nil
		c.candidateIndex = 0
		if c.preeditKana() == "" {
			c.resetComposition()
		}
		return
	}
	c.deleteBeforeCursor()
}

// copyAndClose commits any pending composition, copies the buffer, and
// asks the UI to hide. The buffer is cleared for the next session.
func (e *Engine) copyAndClose() {
	c := &e.main
	if c.roman != "" {
		e.flushPendingRoman()
	}
	if c.composing {
		e.commitCandidate()
	}
	text := string(c.text)
	if text == "" {
		e.status = "Nothing to copy."
		return
	}
	if e.clip == nil {
		e.status = "Copy failed."
		return
	}
	if err := e.clip.Copy(text); err != nil {
		e.status = "Copy failed."
		return
	}
	e.addInputHistory(text)
	e.status = "Copied."
	e.copied = true
	e.closeRequested = true
	e.resetForNewSession()
}

func (e *Engine) resetForNewSession() {
	if e.registerOpen {
		e.closeRegisterModalSilently()
	}
	c := &e.main
	c.resetComposition()
	c.asciiMode = false
	c.wideAscii = false
	c.katakana = ""
	c.text = nil
	c.cursor = 0
}

// PasteClipboard inserts the clipboard text at the cursor, committing any
// pending composition first (the frontend's paste handler).
func (e *Engine) PasteClipboard() {
	if e.clip == nil {
		return
	}
	text, err := e.clip.Read()
	if err != nil {
		return
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	if e.registerOpen {
		r := &e.reg
		if r.roman != "" && !e.flushRegisterRoman() {
			r.roman = ""
		}
		if r.composing {
			e.commitRegisterComposition()
		}
		r.insertText(text)
		return
	}
	c := &e.main
	if c.abbrevMode {
		e.closeAbbrev(c.abbrev)
	}
	if c.roman != "" && !e.flushPendingRoman() {
		pending := c.roman
		c.roman = ""
		if c.composing {
			c.appendComposingKana(pending)
		} else {
			c.insertText(pending)
		}
	}
	if c.composing {
		e.commitCandidate()
	}
	c.insertText(text)
}

// HandleKey applies one key press to whichever buffer has focus.
func (e *Engine) HandleKey(k Key) {
	e.closeRequested = false
	e.copied = false
	if e.registerOpen {
		e.handleRegisterKey(k)
		return
	}
	e.handleMainKey(k)
}

func (e *Engine) handleMainKey(k Key) {
	c := &e.main
	lower := toLowerASCII(k.Key)

	if !c.composing && c.roman == "" && !c.abbrevMode && (k.Key == "Up" || k.Key == "Down") {
		direction := 1
		if k.Key == "Up" {
			direction = -1
		}
		e.showInputHistory(direction)
		return
	}
	e.inputHistoryIndex = -1
	e.inputHistoryDraft = nil

	if k.Ctrl && !k.Alt {
		switch lower {
		case "j":
			if c.asciiMode || c.wideAscii {
				e.enterKanaMode()
				return
			}
			if c.roman != "" && !e.flushPendingRoman() {
				return
			}
			if c.composing {
				e.commitCandidate()
			}
			return
		case "g":
			e.cancelCandidateSelection()
			return
		case "q":
			if c.composing {
				if !e.flushPendingRoman() {
					return
				}
				e.commitKatakana(true)
			} else {
				e.toggleKatakanaMode("han")
			}
			return
		case "v":
			e.PasteClipboard()
			return
		}
	}
	if k.Ctrl || k.Alt {
		return
	}

	if (c.asciiMode || c.wideAscii) && isASCIIPrintable(k.Key) {
		if c.wideAscii {
			c.insertText(toFullWidthASCII(k.Key))
		} else {
			c.insertText(k.Key)
		}
		return
	}

	if k.Key == "Tab" && !k.Shift && c.composing && !c.showingCandidate && c.okuriKey == "" && !c.abbrevMode {
		e.handleCompletion()
		return
	}

	if e.candidateListActive() {
		if labelIndex := indexOfString(listLabels, lower); labelIndex >= 0 {
			target := c.candidateIndex + labelIndex
			if target < len(c.candidates) {
				c.candidateIndex = target
				e.commitCandidate()
			}
			return
		}
	}

	if k.Key == "Escape" {
		if c.abbrevMode {
			e.closeAbbrev("")
			return
		}
		if c.composing || c.roman != "" {
			c.resetComposition()
			return
		}
		e.closeRequested = true
		return
	}

	if k.Key == "/" && !c.composing && c.roman == "" && !c.abbrevMode {
		e.startAbbrev()
		return
	}
	if k.Key == "Backspace" {
		e.handleBackspace()
		return
	}
	if e.handleZCommand(k) {
		return
	}
	if k.Key == " " && c.abbrevMode {
		e.showAbbrevCandidates()
		return
	}
	if e.handleAbbrevPrintable(k) {
		return
	}
	if c.abbrevMode {
		return
	}
	if k.Key == "l" {
		e.enterAsciiMode()
		return
	}
	if k.Key == "L" {
		e.enterWideAsciiMode()
		return
	}
	if k.Key == ";" && !c.composing {
		e.startComposition()
		return
	}
	if k.Key == ";" && c.composing && c.okuriKey == "" && c.preeditKana() != "" {
		c.stickyOkuri = true
		return
	}
	if k.Key == " " && c.composing {
		e.showNextCandidate()
		return
	}
	if c.composing && c.showingCandidate && k.Key == "X" {
		e.purgeCurrentCandidate()
		return
	}
	if lower == "x" && c.composing && c.showingCandidate {
		e.showPreviousCandidate()
		return
	}
	if k.Key == "Enter" {
		if k.Shift {
			if c.roman != "" && !e.flushPendingRoman() {
				return
			}
			if c.composing {
				e.commitCandidate()
			}
			c.insertText("\n")
			return
		}
		if c.composing || c.roman != "" {
			if c.roman != "" && !e.flushPendingRoman() {
				return
			}
			e.commitCandidate()
			return
		}
		e.copyAndClose()
		return
	}
	if lower == "q" && c.composing {
		if !e.flushPendingRoman() {
			return
		}
		e.commitKatakana(false)
		return
	}
	if k.Key == "q" && !c.composing {
		e.toggleKatakanaMode("zen")
		return
	}
	if e.handlePrefixSuffixConversion(k) {
		return
	}
	if e.handlePrintable(k) {
		return
	}
	if e.handleLiteralASCII(k) {
		return
	}
	if !c.composing && c.roman == "" {
		e.handleCaretKey(c, k.Key)
	}
}

// handleCaretKey moves the caret through committed text. The browser
// textarea did this natively; here the engine owns the caret.
func (e *Engine) handleCaretKey(c *composer, key string) {
	switch key {
	case "Left":
		c.cursor--
	case "Right":
		c.cursor++
	case "Home":
		c.cursor = 0
	case "End":
		c.cursor = len(c.text)
	case "Delete":
		c.deleteAfterCursor()
	}
	c.clampCursor()
}

// SetCursor moves the caret to a display offset (a mouse click). Offsets
// inside the preedit collapse to its start, as in the frontend.
func (e *Engine) SetCursor(displayPos int) {
	if e.registerOpen {
		return
	}
	c := &e.main
	c.clampCursor()
	preedit := []rune(e.currentPreeditText())
	start := c.cursor
	end := start + len(preedit)
	switch {
	case len(preedit) == 0 || displayPos <= start:
		c.cursor = displayPos
	case displayPos >= end:
		c.cursor = displayPos - len(preedit)
	default:
		c.cursor = start
	}
	c.clampCursor()
}

// ---- small helpers -----------------------------------------------------------

func containsString(list []string, s string) bool { return indexOfString(list, s) >= 0 }

func indexOfString(list []string, s string) int {
	for i, item := range list {
		if item == s {
			return i
		}
	}
	return -1
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
