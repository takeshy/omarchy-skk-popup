package skk

// composer holds the per-buffer input state. The popup has two of them:
// the main clipboard input and the word-registration field, which share
// the romaji layer but have their own key handling.
type composer struct {
	text   []rune
	cursor int

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
}

// insertText inserts at the cursor (the frontend's replaceSelectedText).
func (c *composer) insertText(text string) {
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
	c.clampCursor()
	if c.cursor == 0 {
		return
	}
	c.text = append(c.text[:c.cursor-1:c.cursor-1], c.text[c.cursor:]...)
	c.cursor--
}

func (c *composer) deleteAfterCursor() {
	c.clampCursor()
	if c.cursor >= len(c.text) {
		return
	}
	c.text = append(c.text[:c.cursor:c.cursor], c.text[c.cursor+1:]...)
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
