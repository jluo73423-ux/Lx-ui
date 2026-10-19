package config

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestReleaseRepoPrecedence(t *testing.T) {
	t.Setenv("LXUI_REPO", "")
	old := releaseRepo
	t.Cleanup(func() { releaseRepo = old })

	releaseRepo = ""
	if got := GetReleaseRepo(); got != repoFromFile() {
		t.Errorf("default = %q, want embedded %q", got, repoFromFile())
	}
	releaseRepo = "build/Override"
	if got := GetReleaseRepo(); got != "build/Override" {
		t.Errorf("ldflags override = %q", got)
	}
	t.Setenv("LXUI_REPO", "env/Lx-ui")
	if got := GetReleaseRepo(); got != "env/Lx-ui" {
		t.Errorf("env override = %q", got)
	}
	if !IsReleaseRepoConfigured() {
		t.Error("env/Lx-ui should be a configured repo")
	}
	if got := GetReleasesAPIURL(); got != "https://api.github.com/repos/env/Lx-ui/releases" {
		t.Errorf("releases API = %q", got)
	}
	if got := GetRawBaseURL(); got != "https://raw.githubusercontent.com/env/Lx-ui" {
		t.Errorf("raw base = %q", got)
	}
}

func TestPlaceholderRepoIsNotConfigured(t *testing.T) {
	t.Setenv("LXUI_REPO", "<ORG>/Lx-ui")
	if IsReleaseRepoConfigured() {
		t.Error("placeholder must not count as configured")
	}
}

func TestDisplayName(t *testing.T) {
	if got := GetDisplayName(); got != "Lx-ui" {
		t.Errorf("GetDisplayName() = %q", got)
	}
}

func TestEnvAliases(t *testing.T) {
	t.Setenv("LXUI_TEST_ALIAS", "new")
	t.Setenv("XUI_TEST_ALIAS", "old")
	t.Setenv("XUI_TEST_ONLY_LEGACY", "1")
	t.Setenv("LXUI_REPO", "a/b")
	t.Setenv("XUI_REPO", "")
	ApplyEnvAliases()
	if got := os.Getenv("XUI_TEST_ALIAS"); got != "new" {
		t.Errorf("LXUI_ must win, XUI_TEST_ALIAS = %q", got)
	}
	if got := os.Getenv("XUI_REPO"); got != "" {
		t.Errorf("LXUI_REPO must not be aliased, XUI_REPO = %q", got)
	}
	legacy := LegacyEnvInUse()
	if !slices.Contains(legacy, "XUI_TEST_ONLY_LEGACY") || slices.Contains(legacy, "XUI_TEST_ALIAS") {
		t.Errorf("LegacyEnvInUse() = %v", legacy)
	}
}

func repoFromFile() string { return strings.TrimSpace(repo) }
