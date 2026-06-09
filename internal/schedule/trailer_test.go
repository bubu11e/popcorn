// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package schedule

import (
	"strings"
	"testing"
)

func TestOriginalTitle(t *testing.T) {
	cases := []struct {
		name        string
		title, orig string
		want        string
	}{
		{"foreign film kept", "Le Diable s'habille en Prada 2", "The Devil Wears Prada 2", "The Devil Wears Prada 2"},
		{"identical dropped", "Pour le plaisir", "Pour le plaisir", ""},
		{"case-insensitive match dropped", "Michael", "MICHAEL", ""},
		{"whitespace trimmed and matched", "Michael", "  Michael ", ""},
		{"blank dropped", "Un film", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := originalTitle(tc.title, tc.orig); got != tc.want {
				t.Errorf("originalTitle(%q, %q) = %q, want %q", tc.title, tc.orig, got, tc.want)
			}
		})
	}
}

func TestTrailerSearchURL(t *testing.T) {
	t.Run("foreign film biases towards VOSTFR", func(t *testing.T) {
		got := trailerSearchURL("Le Diable s'habille en Prada 2", "The Devil Wears Prada 2")
		if !strings.HasPrefix(got, "https://www.youtube.com/results?search_query=") {
			t.Fatalf("unexpected base: %q", got)
		}
		if !strings.Contains(got, "VOSTFR") {
			t.Errorf("foreign film should request VOSTFR, got %q", got)
		}
		// Spaces are encoded and the French title drives the query.
		if !strings.Contains(got, "Diable") || strings.Contains(got, " ") {
			t.Errorf("query not properly encoded: %q", got)
		}
	})

	t.Run("french film omits VOSTFR", func(t *testing.T) {
		got := trailerSearchURL("Pour le plaisir", "Pour le plaisir")
		if strings.Contains(got, "VOSTFR") {
			t.Errorf("french film should not request VOSTFR, got %q", got)
		}
	})

	t.Run("missing original title omits VOSTFR", func(t *testing.T) {
		got := trailerSearchURL("Un film", "")
		if strings.Contains(got, "VOSTFR") {
			t.Errorf("blank original title should not request VOSTFR, got %q", got)
		}
	})
}
