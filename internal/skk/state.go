package skk

import "encoding/json"

// State is the render model handed to the UI after every request. The UI
// is a pure view of it: display text with the preedit already spliced in,
// the caret offset into that text, and the labels the status bar shows.
type State struct {
	Text   string `json:"text"`
	Cursor int    `json:"cursor"`
	// SelStart/SelEnd are the ordered ends of a Shift-selection over the
	// display text; equal (both = Cursor) when there is no selection.
	SelStart        int    `json:"selStart"`
	SelEnd          int    `json:"selEnd"`
	Mode            string `json:"mode"`
	Candidate       string `json:"candidate"`
	CandidateActive bool   `json:"candidateActive"`
	Status          string `json:"status"`

	Register RegisterState `json:"register"`

	// Close asks the UI to hide the popup; Copied says the clipboard was
	// just written (auto paste is due once the popup is hidden).
	Close  bool `json:"close"`
	Copied bool `json:"copied"`
}

type RegisterState struct {
	Open      bool   `json:"open"`
	Reading   string `json:"reading"`
	Text      string `json:"text"`
	Cursor    int    `json:"cursor"`
	SelStart  int    `json:"selStart"`
	SelEnd    int    `json:"selEnd"`
	Mode      string `json:"mode"`
	Candidate string `json:"candidate"`
	Error     string `json:"error"`
}

// State renders the current engine state.
func (e *Engine) State() State {
	c := &e.main
	c.clampCursor()
	preedit := []rune(e.currentPreeditText())
	display := make([]rune, 0, len(c.text)+len(preedit))
	display = append(display, c.text[:c.cursor]...)
	display = append(display, preedit...)
	display = append(display, c.text[c.cursor:]...)

	cursor := c.cursor + len(preedit)
	s := State{
		Text:     string(display),
		Cursor:   cursor,
		SelStart: cursor,
		SelEnd:   cursor,
		Mode:     e.modeLabel(),
		Status:   e.status,
		Register: e.registerState(),
		Close:    e.closeRequested,
		Copied:   e.copied,
	}
	// A selection only exists over plain committed text (no preedit).
	if len(preedit) == 0 && c.hasSelection() {
		s.SelStart, s.SelEnd = c.selRange()
	}
	if c.composing && c.showingCandidate {
		s.CandidateActive = true
		if e.candidateListActive() {
			s.Candidate = e.candidateListText()
		} else {
			s.Candidate = e.candidateText()
			if annotation := candidateAnnotation(c.currentCandidate()); annotation != "" {
				s.Candidate += " ※" + annotation
			}
		}
	}
	return s
}

func (e *Engine) modeLabel() string {
	c := &e.main
	switch {
	case c.wideAscii:
		return "SKK 全英"
	case c.asciiMode:
		return "SKK OFF"
	case c.abbrevMode:
		return "SKK 略語"
	case c.composing && c.showingCandidate:
		return "SKK 候補"
	case c.composing:
		return "SKK 変換"
	case c.katakana == "han":
		return "SKK 半ｶﾅ"
	case c.katakana == "zen":
		return "SKK カナ"
	}
	return "SKK かな"
}

func (e *Engine) registerState() RegisterState {
	r := &e.reg
	if !e.registerOpen {
		return RegisterState{}
	}
	r.clampCursor()
	preedit := []rune(e.registerPreeditText())
	display := make([]rune, 0, len(r.text)+len(preedit))
	display = append(display, r.text[:r.cursor]...)
	display = append(display, preedit...)
	display = append(display, r.text[r.cursor:]...)

	rcursor := r.cursor + len(preedit)
	rs := RegisterState{
		Open:     true,
		Reading:  e.registerKey,
		Text:     string(display),
		Cursor:   rcursor,
		SelStart: rcursor,
		SelEnd:   rcursor,
		Error:    e.registerError,
	}
	if len(preedit) == 0 && r.hasSelection() {
		rs.SelStart, rs.SelEnd = r.selRange()
	}
	switch {
	case r.wideAscii:
		rs.Mode = "SKK 全英"
	case r.asciiMode:
		rs.Mode = "SKK OFF"
	case r.showingCandidate:
		rs.Mode = "SKK 候補"
	case r.composing:
		rs.Mode = "SKK 変換"
	case r.katakana == "han":
		rs.Mode = "SKK 半ｶﾅ"
	case r.katakana == "zen":
		rs.Mode = "SKK カナ"
	default:
		rs.Mode = "SKK かな"
	}
	if r.showingCandidate {
		raw := r.currentCandidate()
		rs.Candidate = candidateWord(raw)
		if annotation := candidateAnnotation(raw); annotation != "" {
			rs.Candidate += " ※" + annotation
		}
	}
	return rs
}

// Text returns the committed buffer (tests and diagnostics).
func (e *Engine) Text() string { return string(e.main.text) }

// RegisterOpen reports whether the registration dialog is showing.
func (e *Engine) RegisterOpen() bool { return e.registerOpen }

// Copy performs the Copy button: commit, copy, and request close.
func (e *Engine) Copy() {
	e.closeRequested = false
	e.copied = false
	e.copyAndClose()
}

// RequestClose is the Close button / click outside: hide without copying.
func (e *Engine) RequestClose() {
	e.closeRequested = true
	e.copied = false
}

// ClearFlags resets the one-shot Close/Copied flags between requests.
func (e *Engine) ClearFlags() {
	e.closeRequested = false
	e.copied = false
}

func parseInputHistory(raw string) []string {
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil
	}
	filtered := list[:0]
	for _, entry := range list {
		if entry != "" {
			filtered = append(filtered, entry)
		}
	}
	if len(filtered) > inputHistoryLimit {
		filtered = filtered[len(filtered)-inputHistoryLimit:]
	}
	return filtered
}

func marshalStrings(list []string) string {
	if list == nil {
		list = []string{}
	}
	data, err := json.Marshal(list)
	if err != nil {
		return "[]"
	}
	return string(data)
}
