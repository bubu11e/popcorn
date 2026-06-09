// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package web

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Comédie":            "comedie",
		"Drame":              "drame",
		"Science fiction":    "science-fiction",
		"Épouvante-horreur":  "epouvante-horreur",
		"  Romance  ":        "romance",
		"Comédie dramatique": "comedie-dramatique",
		"":                   "",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
