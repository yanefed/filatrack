package domain

import (
	"strings"
	"unicode"
)

func NewSpoolDisplayCode(spool *Spool, uniqueSource string) string {
	return strings.Join([]string{
		codePart(string(spool.Material), "MAT", 3),
		codePart(spool.Color, "COL", 3),
		codePart(spool.Manufacturer, "MFR", 3),
		uniquePart(uniqueSource),
	}, "-")
}

func codePart(value, fallback string, length int) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
		if b.Len() >= length {
			break
		}
	}
	for b.Len() < length {
		b.WriteByte(fallback[b.Len()%len(fallback)])
	}
	return b.String()
}

func uniquePart(value string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if len(s) >= 4 {
		return s[len(s)-4:]
	}
	return codePart(s, "0000", 4)
}
