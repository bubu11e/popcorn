// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package schedule

import (
	"net/url"
	"strings"
)

// originalTitle returns the international title to surface alongside the French
// one, or "" when there is nothing worth showing. Allocine repeats the French
// title in originalTitle for domestic films, and a case-insensitive match is
// treated as identical so we never print a redundant near-duplicate line.
func originalTitle(title, original string) string {
	original = strings.TrimSpace(original)
	if original == "" || strings.EqualFold(original, strings.TrimSpace(title)) {
		return ""
	}
	return original
}

// trailerSearchURL builds a YouTube search link for a movie's trailer. Allocine
// does not expose a trailer URL via its showtimes API, so we point at a search
// that reliably surfaces the official trailer as the top result.
//
// When the film is foreign (it has a distinct original title), we bias the
// search towards the VOSTFR cut (original audio, French subtitles). For French
// films that token is meaningless, so it is omitted.
func trailerSearchURL(title, original string) string {
	query := title + " bande annonce"
	if originalTitle(title, original) != "" {
		query += " VOSTFR"
	}
	return "https://www.youtube.com/results?search_query=" + url.QueryEscape(query)
}
