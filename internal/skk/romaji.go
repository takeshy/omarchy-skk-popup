// Package skk implements the SKK input state machine used by skk-popup.
//
// It is a direct port of the JavaScript engine that powered the Wails
// popup (frontend/src/skk_engine.js + main.js in takeshy/skk-popup): the
// same romaji table, the same key bindings, and the same candidate,
// registration, and history behaviour, so a user switching between the
// two sees no difference in how text is entered.
package skk

import "strings"

const (
	henkanPrefix = "▽"
	abbrevPrefix = "▽/"
	okuriMarker  = "*"
)

var kanaTable = map[string]string{
	"-": "ー", ",": "、", ".": "。", "[": "「", "]": "」",
	"a": "あ", "i": "い", "u": "う", "e": "え", "o": "お",
	"xa": "ぁ", "xi": "ぃ", "xu": "ぅ", "xe": "ぇ", "xo": "ぉ",
	"ka": "か", "ki": "き", "ku": "く", "ke": "け", "ko": "こ",
	"sa": "さ", "shi": "し", "si": "し", "su": "す", "se": "せ", "so": "そ",
	"ta": "た", "chi": "ち", "ti": "ち", "tsu": "つ", "tu": "つ", "te": "て", "to": "と",
	"na": "な", "ni": "に", "nu": "ぬ", "ne": "ね", "no": "の",
	"ha": "は", "hi": "ひ", "fu": "ふ", "hu": "ふ", "he": "へ", "ho": "ほ",
	"ma": "ま", "mi": "み", "mu": "む", "me": "め", "mo": "も",
	"ya": "や", "yu": "ゆ", "yo": "よ",
	"xya": "ゃ", "xyu": "ゅ", "xyo": "ょ",
	"ra": "ら", "ri": "り", "ru": "る", "re": "れ", "ro": "ろ",
	"wa": "わ", "wi": "うぃ", "we": "うぇ", "wo": "を", "nn": "ん", "xtu": "っ",
	"ga": "が", "gi": "ぎ", "gu": "ぐ", "ge": "げ", "go": "ご",
	"za": "ざ", "ji": "じ", "zi": "じ", "zu": "ず", "ze": "ぜ", "zo": "ぞ",
	"da": "だ", "di": "ぢ", "du": "づ", "de": "で", "do": "ど",
	"ba": "ば", "bi": "び", "bu": "ぶ", "be": "べ", "bo": "ぼ",
	"pa": "ぱ", "pi": "ぴ", "pu": "ぷ", "pe": "ぺ", "po": "ぽ",
	"kya": "きゃ", "kyu": "きゅ", "kyo": "きょ",
	"sha": "しゃ", "shu": "しゅ", "sho": "しょ",
	"sya": "しゃ", "syu": "しゅ", "syo": "しょ",
	"cha": "ちゃ", "chu": "ちゅ", "che": "ちぇ", "cho": "ちょ",
	"tya": "ちゃ", "tyu": "ちゅ", "tye": "ちぇ", "tyo": "ちょ",
	"nya": "にゃ", "nyu": "にゅ", "nyo": "にょ",
	"hya": "ひゃ", "hyu": "ひゅ", "hyo": "ひょ",
	"mya": "みゃ", "myu": "みゅ", "myo": "みょ",
	"rya": "りゃ", "ryu": "りゅ", "ryo": "りょ",
	"gya": "ぎゃ", "gyu": "ぎゅ", "gyo": "ぎょ",
	"ja": "じゃ", "ju": "じゅ", "jo": "じょ", "je": "じぇ",
	"jya": "じゃ", "jyu": "じゅ", "jyo": "じょ",
	"bya": "びゃ", "byu": "びゅ", "byo": "びょ",
	"pya": "ぴゃ", "pyu": "ぴゅ", "pyo": "ぴょ",
	"fa": "ふぁ", "fi": "ふぃ", "fe": "ふぇ", "fo": "ふぉ",
	"va": "ゔぁ", "vi": "ゔぃ", "vu": "ゔ", "ve": "ゔぇ", "vo": "ゔぉ",
}

// romanPrefixes holds every proper prefix of a romaji key, so a pending
// consonant ("k", "sh", ...) is kept while waiting for its vowel.
var romanPrefixes = func() map[string]bool {
	prefixes := map[string]bool{}
	for key := range kanaTable {
		for n := 1; n < len(key); n++ {
			prefixes[key[:n]] = true
		}
	}
	return prefixes
}()

func isSmallTsuConsonant(b byte) bool { return strings.IndexByte("bcdfghjklmpqrstvwxyz", b) >= 0 }
func isNFollower(b byte) bool         { return strings.IndexByte("aiueoyn", b) >= 0 }

func isUpperASCII(s string) bool { return len(s) == 1 && s[0] >= 'A' && s[0] <= 'Z' }
func isLowerASCII(s string) bool { return len(s) == 1 && s[0] >= 'a' && s[0] <= 'z' }
func isDigit(s string) bool      { return len(s) == 1 && s[0] >= '0' && s[0] <= '9' }

// isASCIIPrintable matches /^[ -~]$/.
func isASCIIPrintable(s string) bool { return len(s) == 1 && s[0] >= ' ' && s[0] <= '~' }

// isHandledPrintableKey lists the keys the romaji layer consumes:
// letters, digits, and , - . ? ' [ ].
func isHandledPrintableKey(s string) bool {
	if len(s) != 1 {
		return false
	}
	c := s[0]
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
		c == ',' || c == '-' || c == '.' || c == '?' || c == '\'' || c == '[' || c == ']'
}

// isAbbrevChar matches [A-Za-z0-9-].
func isAbbrevChar(s string) bool {
	if len(s) != 1 {
		return false
	}
	c := s[0]
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
}

func toKatakana(text string) string {
	runes := []rune(text)
	for i, r := range runes {
		if r >= 0x3041 && r <= 0x3096 {
			runes[i] = r + 0x60
		}
	}
	return string(runes)
}

func toFullWidthASCII(text string) string {
	runes := []rune(text)
	for i, r := range runes {
		switch {
		case r == ' ':
			runes[i] = '　'
		case r > ' ' && r <= '~':
			runes[i] = r + 0xfee0
		}
	}
	return string(runes)
}

var halfKatakanaMap = map[rune]string{
	'ア': "ｱ", 'イ': "ｲ", 'ウ': "ｳ", 'エ': "ｴ", 'オ': "ｵ",
	'カ': "ｶ", 'キ': "ｷ", 'ク': "ｸ", 'ケ': "ｹ", 'コ': "ｺ",
	'サ': "ｻ", 'シ': "ｼ", 'ス': "ｽ", 'セ': "ｾ", 'ソ': "ｿ",
	'タ': "ﾀ", 'チ': "ﾁ", 'ツ': "ﾂ", 'テ': "ﾃ", 'ト': "ﾄ",
	'ナ': "ﾅ", 'ニ': "ﾆ", 'ヌ': "ﾇ", 'ネ': "ﾈ", 'ノ': "ﾉ",
	'ハ': "ﾊ", 'ヒ': "ﾋ", 'フ': "ﾌ", 'ヘ': "ﾍ", 'ホ': "ﾎ",
	'マ': "ﾏ", 'ミ': "ﾐ", 'ム': "ﾑ", 'メ': "ﾒ", 'モ': "ﾓ",
	'ヤ': "ﾔ", 'ユ': "ﾕ", 'ヨ': "ﾖ",
	'ラ': "ﾗ", 'リ': "ﾘ", 'ル': "ﾙ", 'レ': "ﾚ", 'ロ': "ﾛ",
	'ワ': "ﾜ", 'ヲ': "ｦ", 'ン': "ﾝ",
	'ァ': "ｧ", 'ィ': "ｨ", 'ゥ': "ｩ", 'ェ': "ｪ", 'ォ': "ｫ",
	'ッ': "ｯ", 'ャ': "ｬ", 'ュ': "ｭ", 'ョ': "ｮ",
	'ガ': "ｶﾞ", 'ギ': "ｷﾞ", 'グ': "ｸﾞ", 'ゲ': "ｹﾞ", 'ゴ': "ｺﾞ",
	'ザ': "ｻﾞ", 'ジ': "ｼﾞ", 'ズ': "ｽﾞ", 'ゼ': "ｾﾞ", 'ゾ': "ｿﾞ",
	'ダ': "ﾀﾞ", 'ヂ': "ﾁﾞ", 'ヅ': "ﾂﾞ", 'デ': "ﾃﾞ", 'ド': "ﾄﾞ",
	'バ': "ﾊﾞ", 'ビ': "ﾋﾞ", 'ブ': "ﾌﾞ", 'ベ': "ﾍﾞ", 'ボ': "ﾎﾞ",
	'パ': "ﾊﾟ", 'ピ': "ﾋﾟ", 'プ': "ﾌﾟ", 'ペ': "ﾍﾟ", 'ポ': "ﾎﾟ",
	'ヴ': "ｳﾞ",
	'ー': "ｰ", '。': "｡", '、': "､", '「': "｢", '」': "｣", '・': "･",
}

func toHalfWidthKatakana(text string) string {
	var b strings.Builder
	for _, r := range text {
		if half, ok := halfKatakanaMap[r]; ok {
			b.WriteString(half)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func candidateWord(candidate string) string {
	if i := strings.IndexByte(candidate, ';'); i >= 0 {
		return candidate[:i]
	}
	return candidate
}

func candidateAnnotation(candidate string) string {
	if i := strings.IndexByte(candidate, ';'); i >= 0 {
		return candidate[i+1:]
	}
	return ""
}
