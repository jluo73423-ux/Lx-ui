package config

import (
	"os"
	"path/filepath"
	"testing"
)

// copyFile is the workhorse of the legacy data migration (legacy.go);
// these tests pin down its contract.

func TestCopyFileCopiesContents(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.db")
	dst := filepath.Join(dir, "dst.db")

	want := []byte("lx-ui sqlite payload\x00\x01\x02")
	if err := os.WriteFile(src, want, 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile returned error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("dst contents = %q, want %q", got, want)
	}
}

func TestCopyFileMissingSourceReturnsError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "does-not-exist.db")
	dst := filepath.Join(dir, "dst.db")

	if err := copyFile(src, dst); err == nil {
		t.Fatal("copyFile with missing source returned nil error, want error")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Errorf("dst should not be created when source is missing, stat err = %v", err)
	}
}

func TestCopyFileOverwritesDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.db")
	dst := filepath.Join(dir, "dst.db")

	if err := os.WriteFile(src, []byte("new"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.WriteFile(dst, []byte("stale-and-longer"), 0o600); err != nil {
		t.Fatalf("write dst: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile returned error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("dst contents = %q, want %q (truncated overwrite)", got, "new")
	}
}
