package skk

// composer holds the per-buffer input state. The popup has two of them:
// the main clipboard input and the word-registration field, which share
// the romaji layer but have their own key handling.
type composer struct {
	text   []rune
	cursor int
	// selAnchor is the fixed end of a Shift-selection over committed text
	// (-1 when there is no selection). goalCol remembers the target column
	// across consecutive vertical moves (-1 when unset).
	selAnchor int
	goalCol   int

	asciiMode bool
	wideAscii bool
	katakana  string // "", "zen", "han"

	roman      string
	abbrev     string
	abbrevMode bool

	composing   bool
	kana        string
	okuriKey    string
	okuriKana   string
	stickyOkuri bool

	candidates       []string
	candidateIndex   int
	showingCandidate bool

	completionMatches []string
	completionIndex   int
}

func (c *composer) preeditKana() string { return c.kana + c.okuriKana }

func (c *composer) lookupKey() string {
	if c.okuriKey != "" {
		return c.kana + c.okuriKey
	}
	return c.kana
}

func (c *composer) abbrevPreedit() string { return abbrevPrefix + c.abbrev }

func (c *composer) composingPreedit() string {
	if c.okuriKey != "" {
		return henkanPrefix + c.kana + okuriMarker + c.okuriKana
	}
	return henkanPrefix + c.preeditKana()
}

func (c *composer) invalidateCandidates() {
	c.candidates = nil
	c.candidateIndex = 0
}

func (c *composer) appendComposingKana(kana string) {
	if !c.composing {
		return
	}
	if c.okuriKey != "" {
		c.okuriKana += kana
	} else {
		c.kana += kana
	}
	c.invalidateCandidates()
}

// shouldStartOkuri reports whether an upper-case letter typed while a
// reading is being composed marks the start of okurigana.
func (c *composer) shouldStartOkuri(key string) bool {
	return isUpperASCII(key) && c.composing && c.okuriKey == "" && c.kana != ""
}

// consumeRomanChunk converts the longest romaji prefix of the pending
// input to kana. It returns the kana produced ("" when nothing matched
// yet); the kana is also appended to the composition when one is open.
func (c *composer) consumeRomanChunk() string {
	r := toLowerASCII(c.roman)
	if len(r) >= 2 && r[:2] == "n'" {
		c.roman = r[2:]
		c.appendComposingKana("ん")
		return "ん"
	}
	if len(r) >= 2 && r[0] == r[1] && isSmallTsuConsonant(r[0]) {
		c.roman = r[1:]
		c.appendComposingKana("っ")
		return "っ"
	}
	if len(r) == 2 && r[0] == 'n' && !isNFollower(r[1]) {
		c.roman = r[1:]
		c.appendComposingKana("ん")
		return "ん"
	}
	for n := min(3, len(r)); n >= 1; n-- {
		if kana, ok := kanaTable[r[:n]]; ok {
			c.roman = r[n:]
			c.appendComposingKana(kana)
			return kana
		}
	}
	if !romanPrefixes[r] && len(r) > 0 {
		c.roman = r[1:]
	}
	return ""
}

func (c *composer) consumePendingN() string {
	if c.roman != "n" {
		return ""
	}
	c.roman = ""
	c.appendComposingKana("ん")
	return "ん"
}

func (c *composer) clampCursor() {
	if c.cursor < 0 {
		c.cursor = 0
	}
	if c.cursor > len(c.text) {
		c.cursor = len(c.text)
	}
	if c.selAnchor > len(c.text) {
		c.selAnchor = len(c.text)
	}
}

// ---- Shift-selection over committed text --------------------------------

func (c *composer) hasSelection() bool {
	return c.selAnchor >= 0 && c.selAnchor != c.cursor
}

// selRange returns the ordered [start, end) of the current selection.
func (c *composer) selRange() (int, int) {
	a, b := c.selAnchor, c.cursor
	if a > b {
		a, b = b, a
	}
	return a, b
}

func (c *composer) clearSelection() { c.selAnchor = -1 }

// deleteSelection removes the selected range (if any) and returns true.
func (c *composer) deleteSelection() bool {
	if !c.hasSelection() {
		c.selAnchor = -1
		return false
	}
	a, b := c.selRange()
	c.text = append(c.text[:a:a], c.text[b:]...)
	c.cursor = a
	c.selAnchor = -1
	return true
}

// insertText inserts at the cursor, replacing any Shift-selection first
// (the frontend's replaceSelectedText).
func (c *composer) insertText(text string) {
	c.deleteSelection()
	c.goalCol = -1
	c.clampCursor()
	runes := []rune(text)
	next := make([]rune, 0, len(c.text)+len(runes))
	next = append(next, c.text[:c.cursor]...)
	next = append(next, runes...)
	next = append(next, c.text[c.cursor:]...)
	c.text = next
	c.cursor += len(runes)
}

func (c *composer) deleteBeforeCursor() {
	if c.deleteSelection() {
		c.goalCol = -1
		return
	}
	c.clampCursor()
	if c.cursor == 0 {
		return
	}
	c.text = append(c.text[:c.cursor-1:c.cursor-1], c.text[c.cursor:]...)
	c.cursor--
	c.goalCol = -1
}

func (c *composer) deleteAfterCursor() {
	if c.deleteSelection() {
		c.goalCol = -1
		return
	}
	c.clampCursor()
	if c.cursor >= len(c.text) {
		return
	}
	c.text = append(c.text[:c.cursor:c.cursor], c.text[c.cursor+1:]...)
	c.goalCol = -1
}

func (c *composer) resetComposition() {
	c.roman = ""
	c.abbrev = ""
	c.abbrevMode = false
	c.composing = false
	c.kana = ""
	c.okuriKey = ""
	c.okuriKana = ""
	c.stickyOkuri = false
	c.candidates = nil
	c.candidateIndex = 0
	c.showingCandidate = false
	c.completionMatches = nil
	c.completionIndex = 0
}

func (c *composer) currentCandidate() string {
	if c.candidateIndex >= 0 && c.candidateIndex < len(c.candidates) {
		return c.candidates[c.candidateIndex]
	}
	return ""
}

func toLowerASCII(s string) string {
	b := []byte(s)
	for i, ch := range b {
		if ch >= 'A' && ch <= 'Z' {
			b[i] = ch + 32
		}
	}
	return string(b)
}

func trimLastRune(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	return string(runes[:len(runes)-1])
}
