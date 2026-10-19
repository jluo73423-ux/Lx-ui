package config

import (
	_ "embed"
	"os"
	"regexp"
	"strings"
)

// repo is the GitHub "owner/name" of the Lx-ui repository this build updates
// from. It is written by tools/rebrand.sh from LXUI_REPO in tools/rebrand.env.
//
//go:embed repo
var repo string

// releaseRepo optionally overrides the embedded repo at build time:
//
//	go build -ldflags "-X github.com/mhsanaei/3x-ui/v3/internal/config.releaseRepo=acme/Lx-ui"
var releaseRepo string

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})/[A-Za-z0-9._-]{1,100}$`)

// GetReleaseRepo returns the "owner/name" GitHub repository used for panel
// self-update and release lookups. Precedence: the LXUI_REPO environment
// variable, the -ldflags override, then the embedded internal/config/repo.
func GetReleaseRepo() string {
	if v := strings.TrimSpace(os.Getenv("LXUI_REPO")); v != "" {
		return v
	}
	if v := strings.TrimSpace(releaseRepo); v != "" {
		return v
	}
	return strings.TrimSpace(repo)
}

// IsReleaseRepoConfigured reports whether GetReleaseRepo is a real
// "owner/name" value. It is false while the "<ORG>/Lx-ui" placeholder is in
// place, so the panel never sends requests to a bogus URL.
func IsReleaseRepoConfigured() bool {
	return repoPattern.MatchString(GetReleaseRepo())
}

// GetRawBaseURL returns the raw.githubusercontent.com base for the release repo.
func GetRawBaseURL() string {
	return "https://raw.githubusercontent.com/" + GetReleaseRepo()
}

// GetReleasesAPIURL returns the GitHub REST releases endpoint of the release repo.
func GetReleasesAPIURL() string {
	return "https://api.github.com/repos/" + GetReleaseRepo() + "/releases"
}

// GetDisplayName returns the human-facing product name, e.g. "Lx-ui" for the
// "lx-ui" application name.
func GetDisplayName() string {
	n := GetName()
	if n == "" {
		return n
	}
	return strings.ToUpper(n[:1]) + n[1:]
}
