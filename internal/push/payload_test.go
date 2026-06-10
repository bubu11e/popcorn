// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package push

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDigestPayloadSingular(t *testing.T) {
	p := DigestPayload([]string{"Dune"})
	if !strings.HasPrefix(p.Title, "Nouveau film") {
		t.Errorf("title = %q, want singular", p.Title)
	}
	if p.Body != "Dune" {
		t.Errorf("body = %q, want %q", p.Body, "Dune")
	}
	if p.URL != "/" {
		t.Errorf("url = %q, want /", p.URL)
	}
}

func TestDigestPayloadPlural(t *testing.T) {
	p := DigestPayload([]string{"A", "B"})
	if !strings.HasPrefix(p.Title, "Nouveaux films") {
		t.Errorf("title = %q, want plural", p.Title)
	}
	if p.Body != "A, B" {
		t.Errorf("body = %q, want %q", p.Body, "A, B")
	}
}

func TestDigestPayloadCapsLongList(t *testing.T) {
	p := DigestPayload([]string{"A", "B", "C", "D", "E"})
	// Lists the first three, then collapses the remaining two.
	if p.Body != "A, B, C et 2 de plus" {
		t.Errorf("body = %q, want collapsed tail", p.Body)
	}
}

func TestPayloadMarshalRoundTrips(t *testing.T) {
	want := Payload{Title: "t", Body: "b", URL: "/"}
	var got Payload
	if err := json.Unmarshal(want.Marshal(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != want {
		t.Errorf("round-trip = %+v, want %+v", got, want)
	}
}
