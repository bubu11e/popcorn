// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package allocine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const onePageBody = `{
  "message": null,
  "error": false,
  "results": [
    {
      "movie": {
        "internalId": 123,
        "title": "Tout va super",
        "originalTitle": "Everything is great",
        "runtime": "1h 31min",
        "synopsis": "Un film.",
        "genres": [{"translate": "Comédie"}, {"translate": "Romance"}],
        "stats": {"wantToSeeCount": 334},
        "poster": {"url": "https://img/poster.jpg"},
        "credits": [{"person": {"firstName": "Patrick", "lastName": "Cassir"}}],
        "cast": {"edges": [
          {"node": {"actor": {"firstName": "Hakim", "lastName": "Jemili"}}},
          {"node": {"actor": null}}
        ]}
      },
      "showtimes": {"multiple": [{"startsAt": "2026-05-29T14:10:00"}, {"startsAt": "2026-05-29T20:30:00"}]}
    }
  ],
  "pagination": {"page": "1", "totalPages": 1}
}`

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, 2*time.Second, 2, nil)
}

func TestGetShowtimesHappyPath(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, onePageBody)
	})

	got, err := c.GetShowtimes(context.Background(), Theater{ID: "W8560", Name: "Test"}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 showtimes, got %d", len(got))
	}

	m := got[0].Movie
	if m.Title != "Tout va super" || m.WantToSee != 334 {
		t.Errorf("movie fields wrong: %+v", m)
	}
	if m.OriginalTitle != "Everything is great" {
		t.Errorf("originalTitle = %q, want %q", m.OriginalTitle, "Everything is great")
	}
	if m.Director != "Patrick Cassir" {
		t.Errorf("director = %q, want %q", m.Director, "Patrick Cassir")
	}
	if len(m.Cast) != 1 || m.Cast[0] != "Hakim Jemili" {
		t.Errorf("cast = %v, want [Hakim Jemili] (nil actor skipped)", m.Cast)
	}
	if m.Poster != "https://img/poster.jpg" {
		t.Errorf("poster = %q", m.Poster)
	}
}

func TestGetShowtimesPagination(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/p-1/") {
			_, _ = fmt.Fprint(w, `{"message":null,"results":[{"movie":{"internalId":1,"title":"A"},"showtimes":{"x":[{"startsAt":"2026-05-29T10:00:00"}]}}],"pagination":{"page":"1","totalPages":2}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"message":null,"results":[{"movie":{"internalId":2,"title":"B"},"showtimes":{"x":[{"startsAt":"2026-05-29T11:00:00"}]}}],"pagination":{"page":"2","totalPages":2}}`)
	})

	got, err := c.GetShowtimes(context.Background(), Theater{ID: "W8560"}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 showtimes across 2 pages, got %d", len(got))
	}
}

func TestGetShowtimesNoShowtimeIsEmpty(t *testing.T) {
	for _, msg := range []string{"no.showtime.error", "next.showtime.on"} {
		c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, `{"message":%q,"results":[],"pagination":{"page":"1","totalPages":1}}`, msg)
		})
		got, err := c.GetShowtimes(context.Background(), Theater{ID: "X"}, time.Now())
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", msg, err)
		}
		if len(got) != 0 {
			t.Fatalf("%s: want empty, got %d", msg, len(got))
		}
	}
}

func TestGetShowtimesRetriesOn500(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = fmt.Fprint(w, onePageBody)
	})

	got, err := c.GetShowtimes(context.Background(), Theater{ID: "X"}, time.Now())
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 showtimes, got %d", len(got))
	}
	if calls.Load() != 3 {
		t.Fatalf("want 3 attempts (2 failures + success), got %d", calls.Load())
	}
}

func TestGetShowtimesExhaustsRetries(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	_, err := c.GetShowtimes(context.Background(), Theater{ID: "X"}, time.Now())
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
}

func TestGetShowtimesMissingMessageKey(t *testing.T) {
	// A payload without "message" must not panic; it is treated as data.
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"results":[],"pagination":{"page":"1","totalPages":1}}`)
	})
	got, err := c.GetShowtimes(context.Background(), Theater{ID: "X"}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %d", len(got))
	}
}

func TestGetShowtimesDirectorFallback(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"message":null,"results":[{"movie":{"internalId":1,"title":"A","credits":[],"poster":null},"showtimes":{"x":[{"startsAt":"2026-05-29T10:00:00"}]}}],"pagination":{"page":"1","totalPages":1}}`)
	})
	got, err := c.GetShowtimes(context.Background(), Theater{ID: "X"}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Movie.Director != "Inconnu" {
		t.Errorf("director = %q, want Inconnu", got[0].Movie.Director)
	}
	if got[0].Movie.Poster != posterFallback {
		t.Errorf("poster = %q, want fallback", got[0].Movie.Poster)
	}
}
