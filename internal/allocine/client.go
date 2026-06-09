// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package allocine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// These API messages indicate "no screenings", not an error condition.
const (
	msgNoShowtime = "no.showtime.error"
	msgNextOn     = "next.showtime.on"
)

// Client fetches showtimes from allocine.fr.
type Client struct {
	baseURL    string
	httpClient *http.Client
	maxRetries int
	logger     *slog.Logger
}

// NewClient builds a Client. baseURL has no trailing slash (e.g.
// "https://www.allocine.fr"); timeout bounds each HTTP attempt; maxRetries is
// the number of extra attempts on transport errors or 5xx responses.
func NewClient(baseURL string, timeout time.Duration, maxRetries int, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
		maxRetries: maxRetries,
		logger:     logger,
	}
}

// GetShowtimes returns every screening for a theater on a given day, following
// pagination. A response with no screenings yields an empty slice and no error.
func (c *Client) GetShowtimes(ctx context.Context, theater Theater, date time.Time) ([]Showtime, error) {
	var showtimes []Showtime
	datestr := date.Format("2006-01-02")

	for page := 1; ; page++ {
		url := fmt.Sprintf("%s/_/showtimes/theater-%s/d-%s/p-%d/", c.baseURL, theater.ID, datestr, page)

		resp, err := c.fetch(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("fetch page %d: %w", page, err)
		}

		// Empty-result markers are a normal outcome, not a failure.
		if resp.Message != nil && (*resp.Message == msgNoShowtime || *resp.Message == msgNextOn) {
			return showtimes, nil
		}
		if resp.Error != nil {
			if b, ok := resp.Error.(bool); !ok || b {
				return nil, fmt.Errorf("api error for theater %s: %v", theater.ID, resp.Error)
			}
		}

		for _, result := range resp.Results {
			movie := result.Movie.toMovie()
			for _, seances := range result.Showtimes {
				for _, seance := range seances {
					st, err := seance.toShowtime(theater, movie)
					if err != nil {
						c.logger.Warn("skipping malformed showtime",
							"theater", theater.ID, "movie", movie.Title, "error", err)
						continue
					}
					showtimes = append(showtimes, st)
				}
			}
		}

		if page >= resp.Pagination.TotalPages {
			return showtimes, nil
		}
	}
}

// fetch performs a single GET with bounded retries on transport errors and 5xx.
func (c *Client) fetch(ctx context.Context, url string) (*apiResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 500 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := c.doRequest(ctx, url)
		if err != nil {
			lastErr = err
			c.logger.Debug("request attempt failed", "url", url, "attempt", attempt, "error", err)
			continue
		}
		return resp, nil
	}

	return nil, fmt.Errorf("exhausted %d retries: %w", c.maxRetries, lastErr)
}

func (c *Client) doRequest(ctx context.Context, url string) (*apiResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("server error: status %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		// 4xx is not retryable; wrap so callers see a permanent failure.
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, truncate(body, 256))
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return &parsed, nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}
