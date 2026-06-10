// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package web

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/bubu11e/popcorn/internal/push"
)

// PushService is the slice of push behaviour the web layer needs. *push.Notifier
// satisfies it; a nil value (or one reporting Enabled() == false) means push is
// off and the endpoints answer 404 so the client hides its notification UI.
type PushService interface {
	Enabled() bool
	PublicKey() string
	Add(sub push.Subscription) error
	Remove(endpoint string) error
}

// pushEnabled reports whether a configured push service is attached.
func (s *Server) pushEnabled() bool {
	return s.push != nil && s.push.Enabled()
}

// vapidPublicKey returns the VAPID public key the browser needs to subscribe, or
// 404 when push is disabled. The client treats 404 as "hide the button".
func (s *Server) vapidPublicKey(c *gin.Context) {
	if !s.pushEnabled() {
		c.Status(http.StatusNotFound)
		return
	}
	c.String(http.StatusOK, s.push.PublicKey())
}

// subscribe stores a browser PushSubscription posted as JSON.
func (s *Server) subscribe(c *gin.Context) {
	if !s.pushEnabled() {
		c.Status(http.StatusNotFound)
		return
	}
	var sub push.Subscription
	if err := c.ShouldBindJSON(&sub); err != nil || sub.Endpoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription"})
		return
	}
	if err := s.push.Add(sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not store subscription"})
		return
	}
	c.Status(http.StatusCreated)
}

// unsubscribe drops a subscription by endpoint.
func (s *Server) unsubscribe(c *gin.Context) {
	if !s.pushEnabled() {
		c.Status(http.StatusNotFound)
		return
	}
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Endpoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint is required"})
		return
	}
	if err := s.push.Remove(body.Endpoint); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not remove subscription"})
		return
	}
	c.Status(http.StatusNoContent)
}

// serviceWorker serves the embedded service worker from the site root. A worker
// served from /static/ would only control the /static/ scope; serving it from
// "/" with the Service-Worker-Allowed header lets it control the whole origin.
func (s *Server) serviceWorker(c *gin.Context) {
	c.Header("Service-Worker-Allowed", "/")
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/javascript; charset=utf-8", s.swJS)
}
