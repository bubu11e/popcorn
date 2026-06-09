// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package schedule

import "sync"

// Store holds the most recent successful snapshot of per-day movie views,
// indexed by day offset (0 = today). Reads always observe a complete snapshot;
// a failed refresh leaves the previous snapshot in place (serve last-good).
type Store struct {
	mu     sync.RWMutex
	days   [][]MovieView
	loaded bool
}

// NewStore returns an empty store. Until the first successful refresh, Day
// returns an empty slice for every offset.
func NewStore() *Store {
	return &Store{}
}

// Replace atomically swaps in a freshly built snapshot.
func (s *Store) Replace(days [][]MovieView) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.days = days
	s.loaded = true
}

// Day returns the movie views for the given day offset, or an empty slice if
// the offset is out of range or no snapshot has loaded yet.
func (s *Store) Day(delta int) []MovieView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if delta < 0 || delta >= len(s.days) {
		return nil
	}
	return s.days[delta]
}

// Snapshot returns the full per-day snapshot. The outer slice is copied so
// callers can read it without the lock; the inner day slices are shared and
// treated as immutable.
func (s *Store) Snapshot() [][]MovieView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([][]MovieView, len(s.days))
	copy(out, s.days)
	return out
}

// Loaded reports whether at least one snapshot has been successfully stored.
func (s *Store) Loaded() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loaded
}
