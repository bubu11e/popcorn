// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package web

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strings"
	"testing"

	"github.com/bubu11e/popcorn/internal/schedule"
)

func TestLiveIsTheSameProbeAsHealth(t *testing.T) {
	srv := newTestServer(t, schedule.NewStore(), 7)
	for _, path := range []string{"/health", "/live"} {
		rec := doGet(srv, path)
		if rec.Code != http.StatusOK || rec.Body.String() != "OK" {
			t.Errorf("%s = %d %q, want 200 %q", path, rec.Code, rec.Body.String(), "OK")
		}
	}
}

func TestLiveStaysUpBeforeTheFirstRefresh(t *testing.T) {
	// A popcorn that has not reached allocine yet is starting, not broken: only
	// readiness may go red, so an orchestrator holds traffic without a restart.
	srv := newTestServer(t, schedule.NewStore(), 7)
	if rec := doGet(srv, "/live"); rec.Code != http.StatusOK {
		t.Errorf("/live before first refresh = %d, want 200", rec.Code)
	}
	if rec := doGet(srv, "/ready"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("/ready before first refresh = %d, want 503", rec.Code)
	}
}

func TestReadyFollowsTheStore(t *testing.T) {
	store := schedule.NewStore()
	srv := newTestServer(t, store, 7)

	rec := doGet(srv, "/ready")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("/ready (empty store) = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not_ready") {
		t.Errorf("/ready body = %q, want not_ready", rec.Body.String())
	}

	store.Replace([][]schedule.MovieView{{{Title: "Dune"}}})

	rec = doGet(srv, "/ready")
	if rec.Code != http.StatusOK {
		t.Fatalf("/ready (loaded store) = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ready") {
		t.Errorf("/ready body = %q, want ready", rec.Body.String())
	}
}

func TestReadyWithoutAProbeIsAlwaysReady(t *testing.T) {
	srv := newTestServer(t, schedule.NewStore(), 7)
	srv.ready = nil
	if rec := doGet(srv, "/ready"); rec.Code != http.StatusOK {
		t.Errorf("/ready (no probe) = %d, want 200", rec.Code)
	}
}

func TestVersionReportsBuildInfo(t *testing.T) {
	srv := newTestServer(t, schedule.NewStore(), 7)
	for _, path := range []string{"/version", "/api/v1/version"} {
		rec := doGet(srv, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d, want 200", path, rec.Code)
		}
		var info struct {
			Version   string `json:"version"`
			Commit    string `json:"commit"`
			GoVersion string `json:"go_version"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		if info.GoVersion != runtime.Version() {
			t.Errorf("%s go_version = %q, want %q", path, info.GoVersion, runtime.Version())
		}
		if info.Version == "" {
			t.Errorf("%s version is empty", path)
		}
	}
}

func TestMetricsExposesTheHTTPCounter(t *testing.T) {
	srv := newTestServer(t, schedule.NewStore(), 7)
	doGet(srv, "/live")

	rec := doGet(srv, "/metrics")
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	// The probe just served must show up under its route template, which is what
	// proves the middleware is actually mounted on this engine.
	if !strings.Contains(body, `popcorn_http_requests_total{method="GET",route="/live",status="200"}`) {
		t.Errorf("/metrics missing the /live counter, body: %s", body)
	}
	if !strings.Contains(body, "popcorn_http_request_duration_seconds") {
		t.Errorf("/metrics missing the latency histogram, body: %s", body)
	}
}
