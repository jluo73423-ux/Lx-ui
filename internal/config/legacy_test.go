package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func newTestMigrator(t *testing.T) (legacyMigrator, string, string) {
	root := t.TempDir()
	legacy := filepath.Join(root, "etc", "x-ui")
	fresh := filepath.Join(root, "etc", "lx-ui")
	return legacyMigrator{
		legacyDir: legacy,
		newDir:    fresh,
		newName:   "lx-ui",
		serviceUp: func() bool { return false },
		now:       func() time.Time { return time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC) },
	}, legacy, fresh
}

func TestLegacyMigrationCopiesFolder(t *testing.T) {
	m, legacy, fresh := newTestMigrator(t)
	writeFile(t, filepath.Join(legacy, "x-ui.db"), "db", 0o600)
	writeFile(t, filepath.Join(legacy, "x-ui.db-wal"), "wal", 0o600)
	writeFile(t, filepath.Join(legacy, "node_token_key.json"), "key", 0o600)
	writeFile(t, filepath.Join(legacy, "certs", "a.pem"), "pem", 0o644)

	res, err := m.run()
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "copied" || len(res.Files) != 4 {
		t.Fatalf("result = %+v", res)
	}
	if got := readFile(t, filepath.Join(fresh, "lx-ui.db")); got != "db" {
		t.Errorf("lx-ui.db = %q", got)
	}
	if got := readFile(t, filepath.Join(fresh, "lx-ui.db-wal")); got != "wal" {
		t.Errorf("lx-ui.db-wal = %q", got)
	}
	if got := readFile(t, filepath.Join(fresh, "certs", "a.pem")); got != "pem" {
		t.Errorf("certs/a.pem = %q", got)
	}
	info, err := os.Stat(filepath.Join(fresh, "node_token_key.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("node_token_key.json mode = %v, err %v", info, err)
	}
	if !fileExists(filepath.Join(fresh, LegacyMigrationMarker)) {
		t.Error("marker not written")
	}
	// the source is never modified
	if got := readFile(t, filepath.Join(legacy, "x-ui.db")); got != "db" {
		t.Errorf("legacy db changed: %q", got)
	}

	// second run is a no-op
	res, err = m.run()
	if err != nil || res.Action != "none" {
		t.Fatalf("second run = %+v, %v", res, err)
	}
}

func TestLegacyMigrationMarkerPreventsRerun(t *testing.T) {
	m, legacy, fresh := newTestMigrator(t)
	writeFile(t, filepath.Join(legacy, "x-ui.db"), "db", 0o600)
	writeFile(t, filepath.Join(fresh, LegacyMigrationMarker), "x", 0o600)
	res, err := m.run()
	if err != nil || res.Action != "none" {
		t.Fatalf("got %+v, %v", res, err)
	}
}

func TestLegacyMigrationRefusesWhileServiceRuns(t *testing.T) {
	m, legacy, fresh := newTestMigrator(t)
	writeFile(t, filepath.Join(legacy, "x-ui.db"), "db", 0o600)
	m.serviceUp = func() bool { return true }
	if _, err := m.run(); !errors.Is(err, ErrLegacyServiceRunning) {
		t.Fatalf("err = %v, want ErrLegacyServiceRunning", err)
	}
	if fileExists(filepath.Join(fresh, "lx-ui.db")) {
		t.Error("database copied although the legacy service runs")
	}
}

func TestLegacyMigrationSkipsWhenNewDBExists(t *testing.T) {
	m, legacy, fresh := newTestMigrator(t)
	writeFile(t, filepath.Join(legacy, "x-ui.db"), "old", 0o600)
	writeFile(t, filepath.Join(fresh, "lx-ui.db"), "new", 0o600)
	res, err := m.run()
	if err != nil || res.Action != "none" {
		t.Fatalf("got %+v, %v", res, err)
	}
	if got := readFile(t, filepath.Join(fresh, "lx-ui.db")); got != "new" {
		t.Errorf("existing database overwritten: %q", got)
	}
}

func TestLegacyMigrationRenamesInPlace(t *testing.T) {
	m, _, fresh := newTestMigrator(t)
	m.folderFixed = true
	writeFile(t, filepath.Join(fresh, "x-ui.db"), "db", 0o600)
	writeFile(t, filepath.Join(fresh, "x-ui.db-shm"), "shm", 0o600)
	res, err := m.run()
	if err != nil || res.Action != "renamed-in-place" || len(res.Files) != 2 {
		t.Fatalf("got %+v, %v", res, err)
	}
	if readFile(t, filepath.Join(fresh, "lx-ui.db")) != "db" || readFile(t, filepath.Join(fresh, "lx-ui.db-shm")) != "shm" {
		t.Error("in-place copy wrong")
	}
	if !fileExists(filepath.Join(fresh, "x-ui.db")) {
		t.Error("original removed")
	}
}

func TestLegacyMigrationFixedFolderDoesNotCrossFolders(t *testing.T) {
	m, legacy, fresh := newTestMigrator(t)
	m.folderFixed = true
	writeFile(t, filepath.Join(legacy, "x-ui.db"), "db", 0o600)
	res, err := m.run()
	if err != nil || res.Action != "none" || fileExists(filepath.Join(fresh, "lx-ui.db")) {
		t.Fatalf("got %+v, %v", res, err)
	}
}

func TestLegacyMigrationDockerUsesLegacyFolder(t *testing.T) {
	m, legacy, fresh := newTestMigrator(t)
	m.inDocker = true
	writeFile(t, filepath.Join(legacy, "x-ui.db"), "db", 0o600)
	res, err := m.run()
	if err != nil || res.Action != "use-legacy-folder" || res.To != legacy {
		t.Fatalf("got %+v, %v", res, err)
	}
	if readFile(t, filepath.Join(legacy, "lx-ui.db")) != "db" {
		t.Error("in-place copy in the legacy volume missing")
	}
	if fileExists(filepath.Join(fresh, "lx-ui.db")) {
		t.Error("data copied into the container filesystem")
	}
}

func TestLegacyMigrationNothingToDo(t *testing.T) {
	m, _, _ := newTestMigrator(t)
	res, err := m.run()
	if err != nil || res.Action != "none" {
		t.Fatalf("got %+v, %v", res, err)
	}
}

func TestLegacyEnvFilePaths(t *testing.T) {
	for _, p := range GetLegacyEnvFilePaths() {
		if filepath.Base(p) != "x-ui" {
			t.Errorf("unexpected legacy env file %s", p)
		}
	}
}
