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

// newNoRetryClient points at h with retries disabled, so error-path tests do not
// pay the 500ms-per-attempt backoff.
func newNoRetryClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, 2*time.Second, 0, nil)
}

func TestGetShowtimesReportsAPIError(t *testing.T) {
	// "error" carries a payload rather than false: a real failure, not "no
	// screenings", so it must not be swallowed as an empty day.
	c := newNoRetryClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"error":{"code":"boom"},"results":[],"pagination":{"page":"1","totalPages":1}}`)
	})
	_, err := c.GetShowtimes(context.Background(), Theater{ID: "W8560"}, time.Now())
	if err == nil {
		t.Fatal("expected an error when the API reports one")
	}
	if !strings.Contains(err.Error(), "W8560") {
		t.Errorf("error = %v, want the theater id in it", err)
	}
}

func TestGetShowtimesIgnoresErrorFalse(t *testing.T) {
	// The API sends error:false on success; only a truthy value is a failure.
	c := newNoRetryClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, onePageBody)
	})
	if _, err := c.GetShowtimes(context.Background(), Theater{ID: "X"}, time.Now()); err != nil {
		t.Fatalf("error:false must not fail the fetch: %v", err)
	}
}

func TestGetShowtimesSkipsUnparseableStartsAt(t *testing.T) {
	// One bad timestamp must cost that screening only, not the whole theater-day.
	c := newNoRetryClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"message":null,"results":[{"movie":{"internalId":1,"title":"A"},`+
			`"showtimes":{"x":[{"startsAt":"pas une date"},{"startsAt":"2026-05-29T10:00:00"}]}}],`+
			`"pagination":{"page":"1","totalPages":1}}`)
	})

	got, err := c.GetShowtimes(context.Background(), Theater{ID: "X"}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want the one parseable showtime, got %d", len(got))
	}
	if got[0].StartsAt.Hour() != 10 {
		t.Errorf("StartsAt = %v, want the 10:00 screening", got[0].StartsAt)
	}
}

func TestGetShowtimesRejects4xxWithoutRetrying(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
		// Longer than the 256-byte cap, so the wrapped message stays bounded.
		_, _ = w.Write([]byte(strings.Repeat("x", 400)))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 2*time.Second, 0, nil)
	_, err := c.GetShowtimes(context.Background(), Theater{ID: "X"}, time.Now())
	if err == nil {
		t.Fatal("expected an error on 404")
	}
	if calls.Load() != 1 {
		t.Errorf("attempts = %d, want 1: a 4xx is permanent", calls.Load())
	}
	if strings.Count(err.Error(), "x") > 300 {
		t.Errorf("error body not truncated: %d chars", len(err.Error()))
	}
}

func TestGetShowtimes4xxKeepsAShortBodyIntact(t *testing.T) {
	c := newNoRetryClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("rate limited"))
	})
	_, err := c.GetShowtimes(context.Background(), Theater{ID: "X"}, time.Now())
	if err == nil {
		t.Fatal("expected an error on 403")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("error = %v, want the upstream body verbatim when it fits", err)
	}
}

func TestGetShowtimesRejectsMalformedJSON(t *testing.T) {
	c := newNoRetryClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"results": [`)
	})
	if _, err := c.GetShowtimes(context.Background(), Theater{ID: "X"}, time.Now()); err == nil {
		t.Fatal("expected a parse error for a truncated body")
	}
}

func TestGetShowtimesTruncatedBodyIsAnError(t *testing.T) {
	// Content-Length promises more than the handler writes, so the client's read
	// fails mid-body rather than yielding a short but valid payload.
	c := newNoRetryClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "4096")
		_, _ = w.Write([]byte(`{"results":[]}`))
	})
	if _, err := c.GetShowtimes(context.Background(), Theater{ID: "X"}, time.Now()); err == nil {
		t.Fatal("expected an error when the body is cut short")
	}
}

func TestGetShowtimesUnreachableHost(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", 500*time.Millisecond, 0, nil)
	if _, err := c.GetShowtimes(context.Background(), Theater{ID: "X"}, time.Now()); err == nil {
		t.Fatal("expected a transport error against a closed port")
	}
}

func TestGetShowtimesUnbuildableURL(t *testing.T) {
	// A base URL with a control character cannot become a request; the client
	// must report that rather than panic while formatting the path.
	c := NewClient("http://exa\x7fmple.invalid", time.Second, 0, nil)
	if _, err := c.GetShowtimes(context.Background(), Theater{ID: "X"}, time.Now()); err == nil {
		t.Fatal("expected an error for an unbuildable request URL")
	}
}

func TestGetShowtimesStopsRetryingWhenContextIsCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// Retries would sleep 500ms before the second attempt; the context expires
	// first, so the call must return promptly instead of waiting it out.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	c := NewClient(srv.URL, 2*time.Second, 5, nil)
	start := time.Now()
	if _, err := c.GetShowtimes(ctx, Theater{ID: "X"}, time.Now()); err == nil {
		t.Fatal("expected an error once the context is cancelled")
	}
	if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
		t.Errorf("took %v: cancellation should abort the backoff, not ride it out", elapsed)
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
