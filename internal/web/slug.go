// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package web

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// diacriticStripper decomposes accented runes (é -> e + combining accent) and
// drops the combining marks, so French genre labels collapse to ASCII slugs.
var diacriticStripper = transform.Chain(
	norm.NFD,
	runes.Remove(runes.In(unicode.Mn)),
	norm.NFC,
)

// slugify turns a display label like "Comédie" into a stable, accent-proof
// token like "comedie" for use in data attributes and the ?genre= query param.
// Any run of non-alphanumeric characters becomes a single hyphen.
func slugify(s string) string {
	stripped, _, err := transform.String(diacriticStripper, s)
	if err != nil {
		stripped = s
	}

	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(stripped) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen && b.Len() > 0:
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}
