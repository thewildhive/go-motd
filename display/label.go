package display

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// FitInstanceLabel shortens a terminal service label without dropping the closing parenthesis.
func FitInstanceLabel(label string) string {
	if len(label) <= DotLabelWidth || !strings.HasSuffix(label, ")") {
		return label
	}
	const suffix = "\u2026)"
	end, width := labelPrefix(label, DotLabelWidth-2)
	tail, _ := labelPrefix(label[end:], DotLabelWidth-width)
	if end+tail == len(label) {
		return label
	}
	return label[:end] + suffix
}

// labelPrefix returns the byte length and terminal width of the prefix that fits.
func labelPrefix(s string, limit int) (end, width int) {
	for end < len(s) {
		n, cells := labelCluster(s[end:])
		if width+cells > limit {
			break
		}
		end += n
		width += cells
	}
	return end, width
}

// Terminals vary on emoji and ambiguous widths. Count ambiguous characters as
// narrow and keep common combining, flag and joined emoji sequences together.
func labelCluster(s string) (int, int) {
	base, n := utf8.DecodeRuneInString(s)
	width := runeCells(base)
	if base >= 0x1f1e6 && base <= 0x1f1ff {
		r, size := utf8.DecodeRuneInString(s[n:])
		if r >= 0x1f1e6 && r <= 0x1f1ff {
			n += size
			width = 2
		}
	}
	for n < len(s) && s[n] >= utf8.RuneSelf {
		r, size := utf8.DecodeRuneInString(s[n:])
		switch {
		case r == 0xfe0f || r == 0x20e3: // emoji presentation or keycap
			width = 2
		case r == 0x200d && emojiBase(base):
			joined, extra := utf8.DecodeRuneInString(s[n+size:])
			if emojiBase(joined) {
				size += extra
				width = 2
			}
		case !zeroWidth(r):
			return n, width
		}
		n += size
	}
	return n, width
}

func emojiBase(r rune) bool {
	return r >= 0x1f000 && r <= 0x1faff || r >= 0x2600 && r <= 0x27bf
}

func zeroWidth(r rune) bool {
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf) || r >= 0x1f3fb && r <= 0x1f3ff
}

func runeCells(r rune) int {
	switch {
	case r < 0x20 || r >= 0x7f && r < 0xa0:
		return 0
	case r < utf8.RuneSelf:
		return 1
	case zeroWidth(r):
		return 0
	case r >= 0x1100 && r <= 0x115f, r == 0x2329 || r == 0x232a,
		r >= 0x2e80 && r <= 0xa4cf && r != 0x303f, r >= 0xac00 && r <= 0xd7a3,
		r >= 0xf900 && r <= 0xfaff, r >= 0xfe10 && r <= 0xfe19, r >= 0xfe30 && r <= 0xfe6f,
		r >= 0xff01 && r <= 0xff60, r >= 0xffe0 && r <= 0xffe6,
		r >= 0x1f1e6 && r <= 0x1f1ff, r >= 0x1f300 && r <= 0x1faff,
		r >= 0x20000 && r <= 0x3fffd:
		return 2
	}
	return 1
}
