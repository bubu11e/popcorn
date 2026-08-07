// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMiddlewareLabelsByRouteTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(Middleware())
	engine.GET("/movie/:slug", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, path := range []string{"/movie/dune", "/movie/alien"} {
		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	// Two different slugs must share one series, or the slug becomes unbounded
	// label cardinality.
	if got := testutil.ToFloat64(
		httpRequests.WithLabelValues("/movie/:slug", "GET", "200"),
	); got != 2 {
		t.Errorf("requests on the route template = %v, want 2", got)
	}
}

func TestMiddlewareSeparatesStatusAndMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(Middleware())
	engine.GET("/push/subscribe", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.POST("/push/subscribe", func(c *gin.Context) { c.Status(http.StatusCreated) })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/push/subscribe", nil))
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/push/subscribe", nil))

	if got := testutil.ToFloat64(
		httpRequests.WithLabelValues("/push/subscribe", "POST", "201"),
	); got != 1 {
		t.Errorf("POST 201 count = %v, want 1", got)
	}
	if got := testutil.ToFloat64(
		httpRequests.WithLabelValues("/push/subscribe", "GET", "200"),
	); got != 1 {
		t.Errorf("GET 200 count = %v, want 1", got)
	}
}

func TestUnmatchedRequestsAreBucketed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(Middleware())

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/nothing-here", nil))

	if got := testutil.ToFloat64(
		httpRequests.WithLabelValues("unmatched", "GET", "404"),
	); got != 1 {
		t.Errorf("unmatched requests = %v, want 1", got)
	}
}

func TestMiddlewareObservesLatency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(Middleware())
	engine.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })

	before := testutil.CollectAndCount(httpDuration)
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))

	if after := testutil.CollectAndCount(httpDuration); after <= before {
		t.Errorf("duration series = %d, want more than %d", after, before)
	}
}

func TestBuildInfoIsAConstantOneCarryingLabels(t *testing.T) {
	BuildInfo.WithLabelValues("dev", "abc123", "go1.26").Set(1)

	if got := testutil.ToFloat64(BuildInfo.WithLabelValues("dev", "abc123", "go1.26")); got != 1 {
		t.Errorf("build_info = %v, want 1", got)
	}
}

func TestReadyTracksReadiness(t *testing.T) {
	Ready.Set(0)
	if got := testutil.ToFloat64(Ready); got != 0 {
		t.Errorf("ready = %v, want 0", got)
	}
	Ready.Set(1)
	if got := testutil.ToFloat64(Ready); got != 1 {
		t.Errorf("ready = %v, want 1", got)
	}
}
