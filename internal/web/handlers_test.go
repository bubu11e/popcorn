// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/bubu11e/popcorn/internal/schedule"
)

const baseTmpl = `{{define "base"}}<html><body>{{block "body" .}}{{end}}</body></html>{{end}}`
const indexTmpl = `{{define "body"}}` +
	`{{range .dates}}<span class="day{{if .Choisi}} sel{{end}}">{{.Index}}</span>{{end}}` +
	`{{range .genres}}<button class="genre" data-genre="{{.Slug}}">{{.Label}}</button>{{end}}` +
	`{{range $i, $day := .days}}<section data-day="{{$i}}"{{if ne $i $.selected}} hidden{{end}}>` +
	`{{range $day}}<article data-genres="{{genreSlugs .Genres}}"><h3>{{.Title}}</h3></article>{{end}}</section>{{end}}{{end}}`

func newTestServer(t *testing.T, store *schedule.Store, days int) *Server {
	t.Helper()
	return newTestServerWithPush(t, store, days, nil)
}

func newTestServerWithPush(t *testing.T, store *schedule.Store, days int, push PushService) *Server {
	t.Helper()
	templates := fstest.MapFS{
		"base.html":  {Data: []byte(baseTmpl)},
		"index.html": {Data: []byte(indexTmpl)},
	}
	static := fstest.MapFS{
		"placeholder": {Data: []byte("x")},
		"js/sw.js":    {Data: []byte("// service worker")},
	}

	srv, err := NewServer(store, days, templates, static, push)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	srv.now = func() time.Time { return time.Date(2026, 5, 29, 9, 0, 0, 0, time.Local) }
	return srv
}

func storeWithDays(days int) *schedule.Store {
	s := schedule.NewStore()
	snapshot := make([][]schedule.MovieView, days)
	for i := range snapshot {
		snapshot[i] = []schedule.MovieView{{Title: fmt.Sprintf("Day%d", i)}}
	}
	s.Replace(snapshot)
	return s
}

func doGet(srv *Server, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	rec := doGet(newTestServer(t, schedule.NewStore(), 7), "/health")
	if rec.Code != http.StatusOK || rec.Body.String() != "OK" {
		t.Errorf("health = %d %q", rec.Code, rec.Body.String())
	}
}

func TestHomeRendersAllDaysSelectedVisible(t *testing.T) {
	srv := newTestServer(t, storeWithDays(7), 7)
	rec := doGet(srv, "/?delta=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	// Every day ships (instant client-side switching)...
	for i := 0; i < 7; i++ {
		if !strings.Contains(body, fmt.Sprintf("<h3>Day%d</h3>", i)) {
			t.Errorf("expected Day%d to be rendered", i)
		}
	}
	// ...but only the selected panel is visible; the rest are hidden.
	if !strings.Contains(body, `<section data-day="2"><article data-genres=""><h3>Day2</h3>`) {
		t.Errorf("selected day 2 should be visible, body: %s", body)
	}
	if !strings.Contains(body, `<section data-day="0" hidden>`) {
		t.Errorf("unselected day 0 should be hidden, body: %s", body)
	}
}

func TestHomeClampsDelta(t *testing.T) {
	srv := newTestServer(t, storeWithDays(7), 7)

	for _, tc := range []struct {
		query string
		want  string // the visible (non-hidden) selected panel
	}{
		{"/?delta=99", `<section data-day="6"><article data-genres=""><h3>Day6</h3>`},  // clamped to days-1
		{"/?delta=-5", `<section data-day="0"><article data-genres=""><h3>Day0</h3>`},  // clamped to 0
		{"/?delta=abc", `<section data-day="0"><article data-genres=""><h3>Day0</h3>`}, // non-numeric -> 0
		{"/", `<section data-day="0"><article data-genres=""><h3>Day0</h3>`},           // missing -> 0
	} {
		rec := doGet(srv, tc.query)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", tc.query, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("%s: want %s in body", tc.query, tc.want)
		}
	}
}

func TestHomeMarksSelectedDate(t *testing.T) {
	srv := newTestServer(t, storeWithDays(7), 7)
	rec := doGet(srv, "/?delta=3")
	// The calendar strip marks index 3 as selected.
	if !strings.Contains(rec.Body.String(), `<span class="day sel">3</span>`) {
		t.Errorf("selected date not marked, body: %s", rec.Body.String())
	}
}

func TestHomeRendersGenreFilter(t *testing.T) {
	store := schedule.NewStore()
	store.Replace([][]schedule.MovieView{
		{
			{Title: "Film A", Genres: []string{"Comédie", "Romance"}},
			{Title: "Film B", Genres: []string{"Drame"}},
		},
	})
	srv := newTestServer(t, store, 1)
	body := doGet(srv, "/").Body.String()

	// The chip bar offers each distinct genre, sorted and slugged.
	for _, chip := range []string{
		`<button class="genre" data-genre="comedie">Comédie</button>`,
		`<button class="genre" data-genre="drame">Drame</button>`,
		`<button class="genre" data-genre="romance">Romance</button>`,
	} {
		if !strings.Contains(body, chip) {
			t.Errorf("missing genre chip %q, body: %s", chip, body)
		}
	}

	// Each card carries its genres as space-separated slugs for client filtering.
	if !strings.Contains(body, `<article data-genres="comedie romance">`) {
		t.Errorf("Film A should carry its genre slugs, body: %s", body)
	}
}

func TestHomeEmptyStore(t *testing.T) {
	// Before any refresh lands, the page still renders (no films, no panic).
	srv := newTestServer(t, schedule.NewStore(), 7)
	rec := doGet(srv, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "<h3>") {
		t.Errorf("expected no films, body: %s", rec.Body.String())
	}
}
