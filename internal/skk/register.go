package skk

// The word-registration dialog: a second SKK-capable input field that
// opens when a reading has no candidates (or the last one is passed).

func (e *Engine) openRegisterModal() {
	c := &e.main
	key := c.lookupKey()
	if key == "" {
		key = c.preeditKana()
	}
	if key == "" {
		return
	}
	e.registerKey = key
	e.registerRead = key
	e.registerOkuri = ""
	if c.okuriKey != "" {
		// Register only the kanji stem; the engine re-appends the okurigana.
		e.registerRead = c.kana + okuriMarker + c.okuriKana
		e.registerOkuri = c.okuriKana
	}
	e.reg = composer{selAnchor: -1, goalCol: -1}
	e.registerError = ""
	e.registerOpen = true
	e.status = "Register a new candidate."
}

func (e *Engine) closeRegisterModalSilently() {
	e.registerOpen = false
	e.registerError = ""
	e.reg.resetComposition()
	e.registerKey = ""
	e.registerRead = ""
	e.registerOkuri = ""
}

// CancelRegister closes the dialog without saving (Escape / Ctrl+G / 閉じる).
func (e *Engine) CancelRegister() { e.closeRegisterModalSilently() }

func (e *Engine) registerPreeditText() string {
	r := &e.reg
	if !r.composing {
		return r.roman
	}
	if r.showingCandidate {
		return candidateWord(r.currentCandidate()) + r.okuriKana
	}
	return r.composingPreedit() + r.roman
}

func (e *Engine) startRegisterComposition() {
	e.reg.resetComposition()
	e.reg.composing = true
}

func (e *Engine) enterRegisterKanaMode() {
	r := &e.reg
	r.asciiMode = false
	r.wideAscii = false
	r.katakana = ""
}

func (e *Engine) enterRegisterAsciiMode(wide bool) {
	r := &e.reg
	if r.roman != "" && !e.flushRegisterRoman() {
		return
	}
	if r.composing {
		e.commitRegisterComposition()
	}
	r.asciiMode = !wide
	r.wideAscii = wide
}

func (e *Engine) handleRegisterToggleCommand() {
	r := &e.reg
	if r.asciiMode || r.wideAscii {
		e.enterRegisterKanaMode()
		return
	}
	if r.roman != "" && !e.flushRegisterRoman() {
		return
	}
	if r.composing {
		e.commitRegisterComposition()
	}
}

// ToggleRegisterMode flips the dialog between kana and ASCII input.
func (e *Engine) ToggleRegisterMode() {
	if !e.registerOpen {
		return
	}
	if e.reg.asciiMode || e.reg.wideAscii {
		e.enterRegisterKanaMode()
	} else {
		e.enterRegisterAsciiMode(false)
	}
}

func (e *Engine) insertRegisterKana(kana string) {
	r := &e.reg
	if r.katakana == "" {
		r.insertText(kana)
		return
	}
	katakana := toKatakana(kana)
	if r.katakana == "han" {
		katakana = toHalfWidthKatakana(katakana)
	}
	r.insertText(katakana)
}

func (e *Engine) convertRegisterRomanChunk() bool {
	r := &e.reg
	kana := r.consumeRomanChunk()
	if kana == "" {
		return false
	}
	if !r.composing {
		e.insertRegisterKana(kana)
	}
	return true
}

func (e *Engine) flushRegisterRoman() bool {
	r := &e.reg
	for guard := 0; r.roman != "" && guard < 8; guard++ {
		before := r.roman
		if e.convertRegisterRomanChunk() {
			continue
		}
		if r.roman != before {
			continue
		}
		if r.roman == "n" {
			kana := r.consumePendingN()
			if !r.composing {
				e.insertRegisterKana(kana)
			}
			return true
		}
		break
	}
	return r.roman == ""
}

func (e *Engine) commitRegisterComposition() {
	r := &e.reg
	if !r.composing {
		return
	}
	text := r.preeditKana()
	if r.showingCandidate {
		if raw := r.currentCandidate(); raw != "" {
			text = candidateWord(raw) + r.okuriKana
		}
	}
	r.insertText(text)
	r.resetComposition()
	e.registerError = ""
}

func (e *Engine) commitRegisterKatakana(half bool) bool {
	r := &e.reg
	if !r.composing || r.preeditKana() == "" {
		return false
	}
	katakana := toKatakana(r.preeditKana())
	if half {
		katakana = toHalfWidthKatakana(katakana)
	}
	r.insertText(katakana)
	r.resetComposition()
	e.registerError = ""
	return true
}

func (e *Engine) toggleRegisterKatakanaMode(kind string) {
	if e.reg.katakana == kind {
		e.reg.katakana = ""
	} else {
		e.reg.katakana = kind
	}
}

func (e *Engine) showNextRegisterCandidate() {
	r := &e.reg
	if !r.composing || !e.flushRegisterRoman() {
		return
	}
	key := r.lookupKey()
	if key == "" {
		return
	}
	if len(r.candidates) == 0 {
		r.candidates = e.dict.Lookup(key)
		r.candidateIndex = 0
		if len(r.candidates) == 0 {
			e.registerError = "候補がありません。変換バッファは維持されます。"
			return
		}
		r.showingCandidate = true
		e.registerError = ""
		return
	}
	if !r.showingCandidate {
		r.candidateIndex = 0
		r.showingCandidate = true
	} else if r.candidateIndex < len(r.candidates)-1 {
		r.candidateIndex++
	} else {
		e.registerError = "これ以上候補がありません。"
	}
}

func (e *Engine) handleRegisterPrintable(k Key) bool {
	r := &e.reg
	ch := k.Key
	if !isHandledPrintableKey(ch) {
		return false
	}
	if r.showingCandidate {
		e.commitRegisterComposition()
	}
	if isUpperASCII(ch) && !r.composing {
		e.startRegisterComposition()
	} else if r.shouldStartOkuri(ch) {
		r.okuriKey = toLowerASCII(ch)
		r.okuriKana = ""
		r.candidates = nil
		r.showingCandidate = false
	}
	if isDigit(ch) && !r.composing {
		r.insertText(ch)
		return true
	}
	r.roman += toLowerASCII(ch)
	for guard := 0; r.roman != "" && guard < 4; guard++ {
		before := r.roman
		if e.convertRegisterRomanChunk() {
			continue
		}
		if r.roman != before {
			continue
		}
		break
	}
	return true
}

// SaveRegister stores the dialog text for the pending reading and commits
// it into the main buffer (the 登録 button / Enter).
func (e *Engine) SaveRegister() {
	if !e.registerOpen {
		return
	}
	r := &e.reg
	if r.roman != "" && !e.flushRegisterRoman() {
		e.registerError = "未確定のローマ字があります。"
		return
	}
	if r.composing {
		e.commitRegisterComposition()
	}
	value := trimSpace(string(r.text))
	if value == "" {
		e.registerError = "登録する単語を入力してください。"
		return
	}
	if e.registerKey == "" {
		e.registerError = "読みが空のため登録できません。"
		return
	}
	e.dict.RegisterWord(e.registerKey, value)
	e.persistUserDict()

	c := &e.main
	next := []string{value}
	for _, candidate := range c.candidates {
		if candidateWord(candidate) != value {
			next = append(next, candidate)
		}
	}
	c.candidates = next
	c.candidateIndex = 0
	c.showingCandidate = true
	e.closeRegisterModalSilently()
	e.commitCandidate()
	e.status = "Registered."
}

func (e *Engine) handleRegisterKey(k Key) {
	r := &e.reg
	lower := toLowerASCII(k.Key)

	if k.Ctrl && !k.Alt {
		editable := !r.composing && r.roman == ""
		switch lower {
		case "o":
			if editable && len(r.text) > 0 {
				r.selAnchor = 0
				r.cursor = len(r.text)
				r.goalCol = -1
			}
			return
		case "a":
			if editable {
				e.moveCaretTo(r, lineStartOfPos(r.text, r.cursor), k.Shift)
			}
			return
		case "e":
			if editable {
				e.moveCaretTo(r, lineEndOfPos(r.text, r.cursor), k.Shift)
			}
			return
		case "f":
			if editable {
				e.moveCaret(r, 1, k.Shift)
			}
			return
		case "b":
			if editable {
				e.moveCaret(r, -1, k.Shift)
			}
			return
		case "k":
			if editable {
				e.killLine(r, 1)
			}
			return
		case "u":
			if editable {
				e.killLine(r, -1)
			}
			return
		case "c":
			if r.hasSelection() && e.clip != nil {
				a, b := r.selRange()
				_ = e.clip.Copy(string(r.text[a:b]))
			}
			return
		case "x":
			if r.hasSelection() && e.clip != nil {
				a, b := r.selRange()
				_ = e.clip.Copy(string(r.text[a:b]))
				r.deleteSelection()
			}
			return
		case "g":
			e.closeRegisterModalSilently()
			return
		case "j":
			e.handleRegisterToggleCommand()
			return
		case "q":
			if r.composing {
				if !e.flushRegisterRoman() {
					return
				}
				e.commitRegisterKatakana(true)
			} else {
				e.toggleRegisterKatakanaMode("han")
			}
			return
		case "v":
			e.PasteClipboard()
			return
		}
	}
	if k.Key == "Escape" {
		e.closeRegisterModalSilently()
		return
	}
	if k.Ctrl || k.Alt {
		return
	}

	if (r.asciiMode || r.wideAscii) && isASCIIPrintable(k.Key) {
		if r.wideAscii {
			r.insertText(toFullWidthASCII(k.Key))
		} else {
			r.insertText(k.Key)
		}
		return
	}
	if k.Key == "l" && !r.composing && r.roman == "" {
		e.enterRegisterAsciiMode(false)
		return
	}
	if k.Key == "L" && !r.composing && r.roman == "" {
		e.enterRegisterAsciiMode(true)
		return
	}
	if lower == "q" && r.composing {
		if !e.flushRegisterRoman() {
			return
		}
		e.commitRegisterKatakana(false)
		return
	}
	if k.Key == "q" && !r.composing {
		e.toggleRegisterKatakanaMode("zen")
		return
	}
	if k.Key == " " && r.composing {
		e.showNextRegisterCandidate()
		return
	}
	if lower == "x" && r.showingCandidate {
		if r.candidateIndex > 0 {
			r.candidateIndex--
		} else {
			r.showingCandidate = false
		}
		e.registerError = ""
		return
	}
	if k.Key == "Backspace" {
		if r.roman != "" || r.composing {
			switch {
			case r.roman != "":
				r.roman = r.roman[:len(r.roman)-1]
			case r.showingCandidate:
				r.showingCandidate = false
			case r.okuriKana != "":
				r.okuriKana = trimLastRune(r.okuriKana)
			case r.okuriKey != "":
				r.okuriKey = ""
			case r.kana != "":
				r.kana = trimLastRune(r.kana)
			}
			if r.composing && r.roman == "" && r.preeditKana() == "" {
				r.resetComposition()
			}
			return
		}
		r.deleteBeforeCursor()
		return
	}
	if k.Key == "Enter" {
		if r.roman != "" && !e.flushRegisterRoman() {
			return
		}
		if r.composing {
			e.commitRegisterComposition()
			return
		}
		e.SaveRegister()
		return
	}
	if e.handleRegisterPrintable(k) {
		return
	}
	// The dialog was a native <input>: keys the SKK layer ignores are
	// typed literally, and the caret keys move through the text.
	if !r.composing && r.roman == "" {
		if isASCIIPrintable(k.Key) {
			r.insertText(k.Key)
			return
		}
		e.handleCaretKey(r, k)
	}
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && isSpaceByte(s[start]) {
		start++
	}
	for end > start && isSpaceByte(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpaceByte(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }
