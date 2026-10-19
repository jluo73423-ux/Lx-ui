package main

// End-to-end check of the legacy data migration with a real SQLite database:
// a folder that only holds the legacy database must come up as an Lx-ui
// database with the same settings and inbounds, the original left untouched.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestLegacyDatabaseMigratesWithData(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dir)
	legacyDB := filepath.Join(dir, "x-ui.db") // lxui:keep

	if err := database.InitDB(legacyDB); err != nil {
		t.Fatalf("init legacy db: %v", err)
	}
	if err := (&service.SettingService{}).SetPort(23456); err != nil {
		t.Fatal(err)
	}
	ib := &model.Inbound{
		UserId: 1, Port: 34567, Protocol: model.VLESS, Tag: "inbound-34567", Enable: true,
		Settings: `{"clients":[{"id":"11111111-2222-3333-4444-555555555555","email":"legacy-user"}],"decryption":"none"}`,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.CloseDB(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(legacyDB)
	if err != nil {
		t.Fatal(err)
	}

	migrateLegacyInstall()

	newDB := config.GetDBPath()
	if filepath.Base(newDB) != config.GetName()+".db" {
		t.Fatalf("unexpected db path %s", newDB)
	}
	if err := database.InitDB(newDB); err != nil {
		t.Fatalf("open migrated db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	if port, err := (&service.SettingService{}).GetPort(); err != nil || port != 23456 {
		t.Fatalf("migrated webPort = %d, %v; want 23456", port, err)
	}
	var got model.Inbound
	if err := database.GetDB().Where("tag = ?", "inbound-34567").First(&got).Error; err != nil {
		t.Fatalf("migrated inbound missing: %v", err)
	}
	if got.Port != 34567 || !bytes.Contains([]byte(got.Settings), []byte("legacy-user")) {
		t.Fatalf("migrated inbound = %+v", got)
	}
	after, err := os.ReadFile(legacyDB)
	if err != nil {
		t.Fatalf("legacy db removed: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("legacy database was modified")
	}

	// A second start must not copy again over the now-live database.
	if err := (&service.SettingService{}).SetPort(24567); err != nil {
		t.Fatal(err)
	}
	if err := database.CloseDB(); err != nil {
		t.Fatal(err)
	}
	migrateLegacyInstall()
	if err := database.InitDB(newDB); err != nil {
		t.Fatal(err)
	}
	if port, _ := (&service.SettingService{}).GetPort(); port != 24567 {
		t.Fatalf("second start re-migrated: port %d, want 24567", port)
	}
}
