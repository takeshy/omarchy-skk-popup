package skk

import (
	"regexp"
	"strings"
)

const kanjiDigits = "〇一二三四五六七八九"

var (
	digitRun     = regexp.MustCompile(`[0-9]+`)
	numericSlot  = regexp.MustCompile(`#([0-9])`)
	hasDigitRe   = regexp.MustCompile(`[0-9]`)
	allDigitsRe  = regexp.MustCompile(`^[0-9]+$`)
	kanjiDigitRs = []rune(kanjiDigits)
)

func toFullWidthDigits(text string) string {
	runes := []rune(text)
	for i, r := range runes {
		if r >= '0' && r <= '9' {
			runes[i] = r + 0xfee0
		}
	}
	return string(runes)
}

func toKanjiDigits(text string) string {
	var b strings.Builder
	for _, r := range text {
		if r >= '0' && r <= '9' {
			b.WriteRune(kanjiDigitRs[r-'0'])
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// toKanjiNumeral renders "1234" as 千二百三十四 (positional kanji numeral).
func toKanjiNumeral(text string) string {
	if !allDigitsRe.MatchString(text) {
		return text
	}
	digits := strings.TrimLeft(text, "0")
	if digits == "" {
		digits = "0"
	}
	if digits == "0" {
		return "〇"
	}

	var groups []string
	for end := len(digits); end > 0; end -= 4 {
		start := end - 4
		if start < 0 {
			start = 0
		}
		groups = append([]string{digits[start:end]}, groups...)
	}
	groupUnits := []string{"", "万", "億", "兆", "京"}
	if len(groups) > len(groupUnits) {
		return toKanjiDigits(digits)
	}

	smallUnits := []string{"", "十", "百", "千"}
	var result strings.Builder
	for i, group := range groups {
		var part strings.Builder
		for j := 0; j < len(group); j++ {
			digit := int(group[j] - '0')
			if digit == 0 {
				continue
			}
			unit := smallUnits[len(group)-1-j]
			if digit == 1 && unit != "" {
				part.WriteString(unit)
			} else {
				part.WriteRune(kanjiDigitRs[digit])
				part.WriteString(unit)
			}
		}
		if part.Len() > 0 {
			result.WriteString(part.String())
			result.WriteString(groupUnits[len(groups)-1-i])
		}
	}
	if result.Len() == 0 {
		return "〇"
	}
	return result.String()
}

// applyNumericCandidate substitutes #0..#3 slots in a dictionary entry with
// the digits captured from the reading, in the SKK numeric conversion
// formats (#0 as-is, #1 full width, #2 kanji digits, #3 kanji numeral).
func applyNumericCandidate(candidate string, numbers []string) string {
	index := 0
	return numericSlot.ReplaceAllStringFunc(candidate, func(match string) string {
		number := ""
		if index < len(numbers) {
			number = numbers[index]
		}
		index++
		switch match[1] {
		case '1':
			return toFullWidthDigits(number)
		case '2':
			return toKanjiDigits(number)
		case '3':
			return toKanjiNumeral(number)
		}
		return number
	})
}
