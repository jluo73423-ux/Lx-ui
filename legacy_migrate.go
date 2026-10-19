package main

// Kept apart from main.go because it names the legacy 3x-ui product on
// purpose; tools/rebrand.sh does not touch this file.

import (
	"errors"
	"log"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

// migrateLegacyInstall copies the data of an existing 3x-ui installation into
// the Lx-ui data folder before the database is opened (see
// config.MigrateLegacyData; the legacy data is never modified).
func migrateLegacyInstall() {
	if legacy := config.LegacyEnvInUse(); len(legacy) > 0 {
		log.Printf("note: legacy XUI_* variables in use (still supported, LXUI_* preferred): %s", strings.Join(legacy, ", "))
	}
	res, err := config.MigrateLegacyData()
	if errors.Is(err, config.ErrLegacyServiceRunning) {
		// Side-by-side install: start with an empty Lx-ui database and leave
		// 3x-ui alone. No data is copied while the old panel is writing to it.
		log.Printf("warning: not migrating 3x-ui data: %v", err)
		return
	}
	if err != nil {
		log.Fatalf("3x-ui -> Lx-ui data migration: %v", err)
	}
	if res.Action != "none" {
		log.Printf("3x-ui -> Lx-ui data migration (%s): %s", res.Action, res.Note)
	}
}
