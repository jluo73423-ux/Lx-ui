package config

import (
	"os"
	"sort"
	"strings"
)

// Lx-ui reads LXUI_* environment variables. The XUI_* names inherited from
// the upstream project keep working as aliases, so existing env files, compose files
// and systemd overrides need no change. Internally the code base still reads
// the XUI_* names: ApplyEnvAliases copies every LXUI_<X> onto XUI_<X>, and an
// LXUI_ value always wins over the XUI_ alias of the same name.

// envAliasesApplied runs the alias pass during package initialisation, before
// any init() function and before any package that imports config reads XUI_*.
var envAliasesApplied = ApplyEnvAliases()

// envNotAliased are LXUI_* variables that have no XUI_ counterpart.
var envNotAliased = map[string]bool{"LXUI_REPO": true}

var legacyEnvInUse []string

// ApplyEnvAliases maps LXUI_* variables onto XUI_* and records which XUI_*
// variables are set without an LXUI_ counterpart. Call it again after loading
// an env file. It returns true so it can initialise a package-level variable.
func ApplyEnvAliases() bool {
	legacy := map[string]bool{}
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		switch {
		case strings.HasPrefix(k, "LXUI_"):
			if envNotAliased[k] {
				continue
			}
			_ = os.Setenv("XUI_"+strings.TrimPrefix(k, "LXUI_"), v)
		case strings.HasPrefix(k, "XUI_"):
			if _, has := os.LookupEnv("L" + k); !has {
				legacy[k] = true
			}
		}
	}
	legacyEnvInUse = legacyEnvInUse[:0]
	for k := range legacy {
		if _, has := os.LookupEnv("L" + k); !has {
			legacyEnvInUse = append(legacyEnvInUse, k)
		}
	}
	sort.Strings(legacyEnvInUse)
	return true
}

// LegacyEnvInUse lists XUI_* variables that are set without an LXUI_
// equivalent. They keep working; the list only feeds a one-line notice.
func LegacyEnvInUse() []string {
	return append([]string(nil), legacyEnvInUse...)
}
