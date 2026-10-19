package config

// Migration of an existing 3x-ui installation to Lx-ui.
//
// This file names the legacy 3x-ui paths on purpose and is excluded from
// tools/rebrand.sh. The migration only ever COPIES: the legacy folder, its
// database and the legacy service are left untouched, so going back to 3x-ui
// is always possible (stop lx-ui, start x-ui).

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	legacyAppName = "x-ui"
	// LegacyMigrationMarker is written into the new data folder after a copy
	// migration so it never runs twice.
	LegacyMigrationMarker = ".migrated-from-x-ui"
)

// ErrLegacyServiceRunning is returned when the legacy x-ui service still runs
// and would keep writing to the database being copied.
var ErrLegacyServiceRunning = errors.New("the legacy 3x-ui service (x-ui.service) is still running; " +
	"stop it first (systemctl disable --now x-ui) or run: install.sh --migrate-from-3x-ui")

// GetLegacyDBFolderPath returns the 3x-ui data folder.
func GetLegacyDBFolderPath() string { return "/etc/x-ui" }

// GetLegacyEnvFilePaths returns the 3x-ui service environment files. They are
// read as a fallback when no Lx-ui env file exists yet.
func GetLegacyEnvFilePaths() []string {
	if runtime.GOOS == "windows" {
		return nil
	}
	return []string{"/etc/default/x-ui", "/etc/conf.d/x-ui", "/etc/sysconfig/x-ui"}
}

// LegacyMigrationResult describes what MigrateLegacyData did.
type LegacyMigrationResult struct {
	Action string   // "none", "copied", "renamed-in-place", "use-legacy-folder"
	From   string   // source folder
	To     string   // destination folder
	Files  []string // files copied (relative to From)
	Note   string   // human-readable explanation
}

// legacyMigrator holds the paths so tests can point it at temporary folders.
type legacyMigrator struct {
	legacyDir   string // /etc/x-ui
	newDir      string // resolved Lx-ui data folder
	newName     string // lx-ui
	folderFixed bool   // XUI_DB_FOLDER set explicitly: never cross folders
	inDocker    bool
	serviceUp   func() bool
	now         func() time.Time
}

// MigrateLegacyData prepares the Lx-ui data folder from a 3x-ui installation.
// Call it once at start-up, after the service env file is loaded and before
// the database is opened.
//
//   - The data folder holds x-ui.db but no lx-ui.db (explicit XUI_DB_FOLDER,
//     Windows, or a reused docker volume): x-ui.db(-wal,-shm) is copied to
//     lx-ui.db in place.
//   - Linux host, default folder, /etc/lx-ui has no database and no marker,
//     /etc/x-ui exists: the whole folder is copied (x-ui.db* renamed to
//     lx-ui.db*), then the marker is written. Refused while x-ui.service runs.
//   - Docker: a container keeps its data in a volume, so instead of copying into
//     the container filesystem the legacy folder is used in place (the
//     process's XUI_DB_FOLDER is set to it) and a notice asks to remount.
func MigrateLegacyData() (LegacyMigrationResult, error) {
	m := legacyMigrator{
		legacyDir:   GetLegacyDBFolderPath(),
		newDir:      GetDBFolderPath(),
		newName:     GetName(),
		folderFixed: os.Getenv("XUI_DB_FOLDER") != "",
		inDocker:    os.Getenv("XUI_IN_DOCKER") == "true",
		serviceUp:   legacyServiceActive,
		now:         time.Now,
	}
	if runtime.GOOS == "windows" {
		m.legacyDir = ""
	}
	res, err := m.run()
	if err == nil && res.Action == "use-legacy-folder" {
		_ = os.Setenv("XUI_DB_FOLDER", res.To)
	}
	return res, err
}

func (m legacyMigrator) run() (LegacyMigrationResult, error) {
	none := LegacyMigrationResult{Action: "none", To: m.newDir}
	if m.newName == legacyAppName {
		return none, nil
	}
	newDB := filepath.Join(m.newDir, m.newName+".db")
	if fileExists(newDB) {
		return none, nil
	}

	// 1. legacy database file inside the resolved folder.
	if fileExists(filepath.Join(m.newDir, legacyAppName+".db")) {
		files, err := copyDBSet(m.newDir, m.newDir, legacyAppName, m.newName)
		if err != nil {
			return none, err
		}
		return LegacyMigrationResult{
			Action: "renamed-in-place", From: m.newDir, To: m.newDir, Files: files,
			Note: fmt.Sprintf("copied %s.db to %s.db in %s; the original is kept", legacyAppName, m.newName, m.newDir),
		}, nil
	}

	// 2. whole legacy folder.
	if m.folderFixed || m.legacyDir == "" || filepath.Clean(m.legacyDir) == filepath.Clean(m.newDir) {
		return none, nil
	}
	if fileExists(filepath.Join(m.newDir, LegacyMigrationMarker)) || !dirHasEntries(m.legacyDir) {
		return none, nil
	}
	if m.inDocker {
		if !fileExists(filepath.Join(m.legacyDir, legacyAppName+".db")) {
			return none, nil
		}
		files, err := copyDBSet(m.legacyDir, m.legacyDir, legacyAppName, m.newName)
		if err != nil {
			return none, err
		}
		return LegacyMigrationResult{
			Action: "use-legacy-folder", From: m.legacyDir, To: m.legacyDir, Files: files,
			Note: fmt.Sprintf("using the legacy data folder %s in place; change the volume mount to %s (or set LXUI_DB_FOLDER=%s)",
				m.legacyDir, m.newDir, m.legacyDir),
		}, nil
	}
	if m.serviceUp != nil && m.serviceUp() {
		return none, ErrLegacyServiceRunning
	}
	files, err := copyTree(m.legacyDir, m.newDir, legacyAppName, m.newName)
	if err != nil {
		return none, fmt.Errorf("copy %s to %s: %w", m.legacyDir, m.newDir, err)
	}
	marker := fmt.Sprintf("source=%s\ncopied_at=%s\nfiles=%d\n", m.legacyDir, m.now().UTC().Format(time.RFC3339), len(files))
	if err := os.WriteFile(filepath.Join(m.newDir, LegacyMigrationMarker), []byte(marker), 0o600); err != nil {
		return none, err
	}
	return LegacyMigrationResult{
		Action: "copied", From: m.legacyDir, To: m.newDir, Files: files,
		Note: fmt.Sprintf("copied 3x-ui data from %s to %s; %s is kept for rollback", m.legacyDir, m.newDir, m.legacyDir),
	}, nil
}

// legacyServiceActive reports whether systemd currently runs x-ui.service.
// systemd keeps /run/systemd/units/invocation:<unit> while a unit is active.
func legacyServiceActive() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := os.Lstat("/run/systemd/units/invocation:" + legacyAppName + ".service")
	return err == nil
}

var dbSuffixes = []string{".db", ".db-wal", ".db-shm"}

// copyDBSet copies <from>.db and its -wal/-shm companions from srcDir to
// <to>.db* in dstDir, without overwriting existing destination files.
func copyDBSet(srcDir, dstDir, from, to string) ([]string, error) {
	var files []string
	for _, suf := range dbSuffixes {
		src := filepath.Join(srcDir, from+suf)
		if !fileExists(src) {
			continue
		}
		dst := filepath.Join(dstDir, to+suf)
		if fileExists(dst) {
			continue
		}
		if err := copyFileMode(src, dst); err != nil {
			return files, err
		}
		files = append(files, from+suf)
	}
	return files, nil
}

// copyTree copies every regular file under src into dst (keeping modes and
// the directory layout), renaming <from>.db* to <to>.db* at the top level.
// Existing destination files are left alone.
func copyTree(src, dst, from, to string) ([]string, error) {
	var files []string
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return nil, err
	}
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil || rel == "." {
			return err
		}
		target := filepath.Join(dst, rel)
		if !strings.Contains(rel, string(filepath.Separator)) {
			for _, suf := range dbSuffixes {
				if rel == from+suf {
					target = filepath.Join(dst, to+suf)
				}
			}
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case !info.Mode().IsRegular():
			return nil // sockets, symlinks, ...
		case fileExists(target):
			return nil
		}
		if err := copyFileMode(path, target); err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	return files, err
}

// copyFile copies src to dst, truncating dst if it exists. dst is not created
// when src cannot be opened.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// copyFileMode is copyFile plus the source file's permission bits.
func copyFileMode(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := copyFile(src, dst); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return os.Chmod(dst, info.Mode().Perm())
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirHasEntries(p string) bool {
	entries, err := os.ReadDir(p)
	return err == nil && len(entries) > 0
}
