// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package web

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/bubu11e/popcorn/metrics"
)

// health is the liveness probe: the process is up and the HTTP server responds.
// It performs no dependency checks, and answers plain text because the
// container HEALTHCHECK and the service worker have always read it that way.
func (s *Server) health(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

// readyz is the readiness probe: 200 once a schedule snapshot has loaded, else
// 503 so an orchestrator holds traffic. Liveness deliberately stays green in
// the meantime -- a popcorn that has not reached allocine yet is starting, not
// broken. It also mirrors the state onto the Ready gauge.
func (s *Server) readyz(c *gin.Context) {
	if s.ready == nil || s.ready() {
		metrics.Ready.Set(1)
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
		return
	}
	metrics.Ready.Set(0)
	c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
}

// version returns build information.
func (s *Server) version(c *gin.Context) {
	c.JSON(http.StatusOK, s.buildInfo)
}
