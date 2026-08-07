// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package version

import (
	"encoding/json"
	"runtime"
	"testing"
)

func TestGetReportsTheRunningToolchain(t *testing.T) {
	info := Get()
	if info.GoVersion != runtime.Version() {
		t.Errorf("go_version = %q, want %q", info.GoVersion, runtime.Version())
	}
	if info.Version == "" {
		t.Error("version is empty")
	}
}

func TestGetReflectsInjectedBuildVars(t *testing.T) {
	origVersion, origCommit := Version, Commit
	t.Cleanup(func() { Version, Commit = origVersion, origCommit })

	Version, Commit = "1.2.3", "0123456789abcdef"
	info := Get()
	if info.Version != "1.2.3" {
		t.Errorf("version = %q, want %q", info.Version, "1.2.3")
	}
	if info.Commit != "0123456789abcdef" {
		t.Errorf("commit = %q, want %q", info.Commit, "0123456789abcdef")
	}
}

func TestInfoMarshalsWithSnakeCaseKeys(t *testing.T) {
	b, err := json.Marshal(Info{Version: "dev", Commit: "abc", GoVersion: "go1.26"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"version":"dev","commit":"abc","go_version":"go1.26"}`
	if string(b) != want {
		t.Errorf("marshal = %s, want %s", b, want)
	}
}

func TestShortCommit(t *testing.T) {
	for in, want := range map[string]string{
		"":             "",
		"abc123":       "abc123",
		"0123456789ab": "0123456789ab",
		"0123456789abcdef0123456789abcdef01234567": "0123456789ab",
	} {
		if got := ShortCommit(in); got != want {
			t.Errorf("ShortCommit(%q) = %q, want %q", in, got, want)
		}
	}
}
