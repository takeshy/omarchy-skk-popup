package skk

// Line helpers for caret navigation over committed text. A "line" is a run
// of runes between '\n' separators; the text always has at least one line.

func lineCount(text []rune) int {
	n := 1
	for _, r := range text {
		if r == '\n' {
			n++
		}
	}
	return n
}

// lineIndexAt returns the 0-based line the position sits on.
func lineIndexAt(text []rune, pos int) int {
	if pos > len(text) {
		pos = len(text)
	}
	n := 0
	for i := 0; i < pos; i++ {
		if text[i] == '\n' {
			n++
		}
	}
	return n
}

// lineBounds returns [start, end) rune offsets of the given line (end is the
// '\n' or len(text)); an out-of-range line collapses to the text end.
func lineBounds(text []rune, line int) (int, int) {
	cur, start := 0, 0
	for i := 0; i < len(text); i++ {
		if text[i] != '\n' {
			continue
		}
		if cur == line {
			return start, i
		}
		cur++
		start = i + 1
	}
	if cur == line {
		return start, len(text)
	}
	return len(text), len(text)
}

func lineStartOfPos(text []rune, pos int) int {
	if pos > len(text) {
		pos = len(text)
	}
	for i := pos - 1; i >= 0; i-- {
		if text[i] == '\n' {
			return i + 1
		}
	}
	return 0
}

func lineEndOfPos(text []rune, pos int) int {
	for i := pos; i < len(text); i++ {
		if text[i] == '\n' {
			return i
		}
	}
	return len(text)
}

func colOfPos(text []rune, pos int) int { return pos - lineStartOfPos(text, pos) }
