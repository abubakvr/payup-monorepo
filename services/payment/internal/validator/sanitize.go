package validator

import (
	"regexp"
	"strings"
	"unicode"
)

// Max lengths for 9PSB open_wallet / DB (conservative).
const (
	MaxLenName      = 100
	MaxLenPhone     = 20
	MaxLenAddress   = 255
	MaxLenRef       = 60
	MaxLenEmail     = 255
	BVNLength       = 11
	NINLength       = 11
	DOBFormatLength = 10 // DD/MM/YYYY
)

// Trim and remove control characters; collapse internal spaces for names/address.
func sanitizeString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		if r == '\t' || r == '\n' || r == '\r' {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	s = strings.TrimSpace(b.String())
	// Collapse multiple spaces
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	if maxLen > 0 && len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

// SanitizeAlphaSpace keeps letters, numbers, spaces, hyphen, apostrophe (names).
func sanitizeAlphaSpace(s string, maxLen int) string {
	s = sanitizeString(s, maxLen)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ' || r == '-' || r == '\'' || r == '.' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// SanitizeDigits returns only digits (for BVN, NIN, phone).
func sanitizeDigits(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if maxLen > 0 && len(out) > maxLen {
		out = out[:maxLen]
	}
	return out
}

// SanitizePhone trims and keeps digits and leading +.
func sanitizePhone(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if maxLen <= 0 {
		maxLen = MaxLenPhone
	}
	var b strings.Builder
	for i, r := range s {
		if i == 0 && r == '+' {
			b.WriteRune('+')
			continue
		}
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > maxLen {
		out = out[:maxLen]
	}
	return out
}

// SanitizeEmail trims and lowercases; no control chars.
func sanitizeEmail(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	s = b.String()
	if len(s) > MaxLenEmail {
		s = s[:MaxLenEmail]
	}
	return s
}
