// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package push

import (
	"encoding/json"
	"fmt"
	"strings"
)

// digestListCap is how many movie titles a notification body lists before it
// collapses the rest into a "+N" tail, keeping the message readable.
const digestListCap = 3

// Payload is the JSON the service worker reads in its push handler to render a
// notification (see static/js/sw.js).
type Payload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

// Marshal serialises the payload for delivery. Marshalling a fixed struct of
// strings cannot fail, so the error is intentionally dropped.
func (p Payload) Marshal() []byte {
	b, _ := json.Marshal(p)
	return b
}

// DigestPayload builds a single French notification summarising newly-released
// movies. The title is pluralised, and the body lists up to digestListCap
// titles, collapsing any overflow into "et N de plus".
func DigestPayload(titles []string) Payload {
	title := "Nouveaux films à l'affiche 🍿"
	if len(titles) == 1 {
		title = "Nouveau film à l'affiche 🍿"
	}

	var body string
	if len(titles) <= digestListCap {
		body = strings.Join(titles, ", ")
	} else {
		extra := len(titles) - digestListCap
		body = fmt.Sprintf("%s et %d de plus", strings.Join(titles[:digestListCap], ", "), extra)
	}

	return Payload{Title: title, Body: body, URL: "/"}
}
