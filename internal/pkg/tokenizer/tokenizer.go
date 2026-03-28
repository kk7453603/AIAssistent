package tokenizer

import (
	"strings"
	"unicode"
)

// TokenizeUnicode splits text into lowercase tokens on non-letter/non-digit boundaries.
// Handles all Unicode scripts (Latin, Cyrillic, CJK, etc.).
func TokenizeUnicode(s string) []string {
	if s == "" {
		return nil
	}
	out := make([]string, 0, 24)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}
