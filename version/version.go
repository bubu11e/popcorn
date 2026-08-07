// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

// Package version exposes build information surfaced at /version and as the
// popcorn_build_info metric.
package version

import "runtime"

// Commit is injected at build time via
// -ldflags "-X github.com/bubu11e/popcorn/version.Commit=<sha>". Empty in local builds.
var Commit = ""

// Version is the human-readable release version. Override via ldflags if you
// cut tagged releases.
var Version = "dev"

// Info is the build information returned by the version endpoints.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
}

// Get returns the current build information.
func Get() Info {
	return Info{Version: Version, Commit: Commit, GoVersion: runtime.Version()}
}

// ShortCommit returns the first 12 characters of a commit SHA, or the whole
// string if it is shorter.
func ShortCommit(c string) string {
	if len(c) > 12 {
		return c[:12]
	}
	return c
}
