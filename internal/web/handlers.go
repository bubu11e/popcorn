// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

// Package web exposes the HTTP server: the home calendar page and a health
// check, rendered with Gin over the in-memory schedule store.
package web

import (
	"hash/crc32"
	"html/template"
	"io/fs"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"

	"github.com/bubu11e/popcorn/internal/schedule"
)

// staticMaxAge is the Cache-Control lifetime for embedded assets (CSS, fonts,
// images). They are versioned with the binary, so a day is comfortably safe.
const staticMaxAge = "public, max-age=86400"

// DateView is one selectable day in the calendar strip.
type DateView struct {
	Jour    string
	Chiffre int
	Mois    string
	Choisi  bool
	Index   int
}

// GenreView is one selectable chip in the genre filter bar. Slug is the
// accent-proof token matched against each card's data-genres attribute and the
// ?genre= query param; Label is the human-readable French genre.
type GenreView struct {
	Label string
	Slug  string
}

// templateFuncs are the helpers exposed to templates. joinComma renders a
// string slice for display; genreSlugs renders the same slice as the
// space-separated slug list a card carries for client-side filtering.
var templateFuncs = template.FuncMap{
	"joinComma": func(parts []string) string { return strings.Join(parts, ", ") },
	"genreSlugs": func(genres []string) string {
		slugs := make([]string, 0, len(genres))
		for _, g := range genres {
			slugs = append(slugs, slugify(g))
		}
		return strings.Join(slugs, " ")
	},
}

// Server holds the dependencies for the HTTP handlers.
type Server struct {
	store    *schedule.Store
	days     int
	engine   *gin.Engine
	now      func() time.Time
	assetVer string // cache-busting token derived from the CSS content
	swJS     []byte // embedded service worker, served from the site root
}

// NewServer builds the Gin engine, parses the embedded templates, and mounts
// the embedded static assets.
func NewServer(store *schedule.Store, days int, templatesFS, staticFS fs.FS) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)

	// http.FileServer types files by extension; .webmanifest is not in Go's
	// default table, so register it for a correct Content-Type.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")

	tmpl, err := template.New("").Funcs(templateFuncs).ParseFS(templatesFS, "*.html")
	if err != nil {
		return nil, err
	}

	swJS, err := fs.ReadFile(staticFS, "js/sw.js")
	if err != nil {
		return nil, err
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	// Compress HTML/CSS responses; skip already-compressed binary assets.
	engine.Use(gzip.Gzip(gzip.DefaultCompression,
		gzip.WithExcludedExtensions([]string{".png", ".jpg", ".jpeg", ".svg", ".ico", ".ttf", ".otf"})))
	engine.Use(staticCache)
	engine.SetHTMLTemplate(tmpl)
	engine.StaticFS("/static", http.FS(staticFS))

	s := &Server{
		store:    store,
		days:     days,
		engine:   engine,
		now:      time.Now,
		assetVer: assetVersion(staticFS),
		swJS:     swJS,
	}

	engine.GET("/", s.home)
	engine.GET("/health", s.health)
	engine.GET("/sw.js", s.serviceWorker)
	return s, nil
}

// serviceWorker serves the embedded service worker from the site root. A worker
// served from /static/ would only control the /static/ scope; serving it from
// "/" with the Service-Worker-Allowed header lets it control the whole origin.
func (s *Server) serviceWorker(c *gin.Context) {
	c.Header("Service-Worker-Allowed", "/")
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/javascript; charset=utf-8", s.swJS)
}

// assetVersion returns a short token that changes whenever the CSS changes, so
// the long-lived static cache is busted on every deploy without weakening it.
func assetVersion(staticFS fs.FS) string {
	b, err := fs.ReadFile(staticFS, "css/main.css")
	if err != nil {
		return "dev"
	}
	return strconv.FormatUint(uint64(crc32.ChecksumIEEE(b)), 16)
}

// Handler returns the underlying http.Handler.
func (s *Server) Handler() http.Handler { return s.engine }

func (s *Server) health(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

func (s *Server) home(c *gin.Context) {
	delta := clamp(intQuery(c, "delta"), 0, s.days-1)

	today := s.now()
	dates := make([]DateView, s.days)
	for i := 0; i < s.days; i++ {
		day := today.AddDate(0, 0, i)
		dates[i] = DateView{
			Jour:    translateDay(day.Weekday()),
			Chiffre: day.Day(),
			Mois:    translateMonth(day.Month()),
			Choisi:  i == delta,
			Index:   i,
		}
	}

	// Ship every day so the client can switch instantly without a round-trip.
	days := make([][]schedule.MovieView, s.days)
	copy(days, s.store.Snapshot())

	// The genre chips filter purely client-side, so we ship the whole catalogue
	// alongside the data and let the browser narrow the cards.
	labels := schedule.CollectGenres(days)
	genres := make([]GenreView, len(labels))
	for i, label := range labels {
		genres[i] = GenreView{Label: label, Slug: slugify(label)}
	}

	c.HTML(http.StatusOK, "base", gin.H{
		"dates":    dates,
		"days":     days,
		"genres":   genres,
		"selected": delta,
		"assetVer": s.assetVer,
	})
}

// staticCache adds a Cache-Control header to embedded static assets.
func staticCache(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/static/") {
		c.Header("Cache-Control", staticMaxAge)
	}
	c.Next()
}

func intQuery(c *gin.Context, key string) int {
	v, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return v
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
