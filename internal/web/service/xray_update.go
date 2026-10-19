package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// Safe Xray binary replacement (#6871).
//
// Upstream stopped Xray, overwrote the binary and restarted; a release that
// cannot start in the panel's environment (e.g. the systemd sandbox) left the
// node with no proxy until someone fixed it over SSH. Lx-ui instead:
//
//  1. downloads and checksum-verifies the archive while Xray keeps running,
//  2. stages the new binary next to the current one,
//  3. validates it in the panel's own environment (so under the same systemd
//     sandbox): "<new> version" must report the requested version and
//     "<new> run -test -c <generated config>" must accept the panel's config.
//     -test builds the full core (the step that fails with "failed to create
//     server > operation not permitted" in #6871) without binding ports,
//  4. only then stops Xray, keeps the current binary as <binary>.bak and swaps,
//  5. starts Xray and confirms it is still running after a grace period;
//     otherwise restores the .bak binary and restarts the previous version.

const (
	xrayValidateTimeout = 20 * time.Second
	xrayStartGrace      = 3 * time.Second
	xrayBackupSuffix    = ".bak"
)

// Hooks overridden by tests; production wires them to the live services.
var (
	xrayUpdateStop   = func(s *ServerService) error { return s.xrayService.StopXray() }
	xrayUpdateStart  = func(s *ServerService) error { return s.startAndConfirmXray() }
	xrayUpdateConfig = func(s *ServerService) ([]byte, error) {
		cfg, err := s.xrayService.GetXrayConfig()
		if err != nil {
			return nil, err
		}
		return json.MarshalIndent(cfg, "", "  ")
	}
)

// startAndConfirmXray force-restarts Xray and fails if the process is gone
// after xrayStartGrace, surfacing the core's own exit error when present.
func (s *ServerService) startAndConfirmXray() error {
	if err := s.xrayService.RestartXray(true); err != nil {
		return err
	}
	time.Sleep(xrayStartGrace)
	if !s.xrayService.IsXrayRunning() {
		if err := s.xrayService.GetXrayErr(); err != nil {
			return fmt.Errorf("xray exited after start: %w (%s)", err, strings.TrimSpace(s.xrayService.GetXrayResult()))
		}
		return errors.New("xray exited right after start")
	}
	return nil
}

// xrayTargetBinary returns the installed binary path and its name inside the
// release archive for this platform.
func xrayTargetBinary() (target, zipName string) {
	if runtime.GOOS == "windows" {
		return filepath.Join(config.GetBinFolderPath(), "xray-windows-amd64.exe"), "xray.exe"
	}
	return xray.GetBinaryPath(), "xray"
}

// stageXrayFromZip extracts zipName from the archive into a temp file in dir
// (same filesystem as the target, so the later swap is an atomic rename).
func stageXrayFromZip(zipPath, zipName, dir string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	src, err := zr.Open(zipName)
	if err != nil {
		return "", fmt.Errorf("xray archive has no %s: %w", zipName, err)
	}
	defer src.Close()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".xray-new-*")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	n, err := io.Copy(tmp, io.LimitReader(src, maxXrayBinaryBytes+1))
	if err != nil {
		return "", err
	}
	if n > maxXrayBinaryBytes {
		return "", fmt.Errorf("xray binary exceeds %d bytes", maxXrayBinaryBytes)
	}
	if err := tmp.Chmod(0o755); err != nil {
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	ok = true
	return path, nil
}

// validateXrayBinary runs the staged binary's version and config test.
// configJSON may be nil to skip the config test.
func validateXrayBinary(ctx context.Context, bin, wantVersion string, configJSON []byte) error {
	vctx, cancel := context.WithTimeout(ctx, xrayValidateTimeout)
	defer cancel()
	out, err := exec.CommandContext(vctx, bin, "version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%q version failed: %w: %s", filepath.Base(bin), err, trimOutput(out))
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 || fields[0] != "Xray" {
		return fmt.Errorf("unexpected version output: %s", trimOutput(out))
	}
	if want := strings.TrimPrefix(wantVersion, "v"); want != "" && fields[1] != want {
		return fmt.Errorf("binary reports Xray %s, expected %s", fields[1], want)
	}
	if configJSON == nil {
		return nil
	}
	cfg, err := os.CreateTemp(filepath.Dir(bin), ".xray-test-*.json")
	if err != nil {
		return err
	}
	cfgPath := cfg.Name()
	defer os.Remove(cfgPath)
	if err := cfg.Chmod(0o600); err != nil {
		_ = cfg.Close()
		return err
	}
	if _, err := cfg.Write(configJSON); err != nil {
		_ = cfg.Close()
		return err
	}
	if err := cfg.Close(); err != nil {
		return err
	}
	tctx, tcancel := context.WithTimeout(ctx, xrayValidateTimeout)
	defer tcancel()
	cmd := exec.CommandContext(tctx, bin, "run", "-test", "-c", cfgPath)
	// Asset lookups (geoip/geosite) resolve against the bin folder, like the
	// real process started by the panel.
	cmd.Dir = filepath.Dir(bin)
	cmd.Env = append(os.Environ(), "XRAY_LOCATION_ASSET="+config.GetBinFolderPath())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("config test failed: %w: %s", err, trimOutput(out))
	}
	return nil
}

// sandboxHint points an EPERM failure at the service sandbox, the suspected
// (unconfirmed) cause of #6871.
func sandboxHint(err error) string {
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "operation not permitted") {
		return ""
	}
	return " (EPERM: under systemd this usually means the lx-ui.service sandbox (SystemCallFilter/CapabilityBoundingSet) blocked a call the new Xray needs; see docs/LX_CHANGES.md, #6871)"
}

func trimOutput(b []byte) string {
	b = bytes.TrimSpace(b)
	const max = 600
	if len(b) > max {
		b = append(b[:max:max], []byte("…")...)
	}
	return string(b)
}

// swapInXrayBinary moves target to target.bak (when present) and the staged
// binary into place. It returns the backup path ("" when there was no
// previous binary).
func swapInXrayBinary(target, staged string) (string, error) {
	backup := ""
	if _, err := os.Stat(target); err == nil {
		backup = target + xrayBackupSuffix
		_ = os.Remove(backup)
		if err := os.Rename(target, backup); err != nil {
			return "", fmt.Errorf("back up current xray: %w", err)
		}
	}
	if err := os.Rename(staged, target); err != nil {
		if backup != "" {
			_ = os.Rename(backup, target)
		}
		return "", fmt.Errorf("install new xray: %w", err)
	}
	return backup, nil
}

// restoreXrayBinary puts the backup back in place of a failed new binary.
func restoreXrayBinary(target, backup string) error {
	if backup == "" {
		return errors.New("no previous xray binary to roll back to")
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(target)
	}
	return os.Rename(backup, target)
}

// installXrayArchive runs steps 2-5 above for an already verified archive.
func (s *ServerService) installXrayArchive(zipPath, version string) error {
	target, zipName := xrayTargetBinary()
	staged, err := stageXrayFromZip(zipPath, zipName, filepath.Dir(target))
	if err != nil {
		return err
	}
	defer os.Remove(staged) // no-op once renamed into place

	cfg, cfgErr := xrayUpdateConfig(s)
	if cfgErr != nil {
		logger.Warning("xray update: cannot generate config for the pre-update test, checking the version only:", cfgErr)
		cfg = nil
	}
	if err := validateXrayBinary(context.Background(), staged, version, cfg); err != nil {
		return fmt.Errorf("xray %s failed pre-update validation; the current version keeps running: %w%s", version, err, sandboxHint(err))
	}

	if err := xrayUpdateStop(s); err != nil {
		logger.Warning("xray update: stop before swap failed:", err)
	}
	backup, err := swapInXrayBinary(target, staged)
	if err != nil {
		if startErr := xrayUpdateStart(s); startErr != nil {
			logger.Error("xray update: restarting the previous version failed:", startErr)
		}
		return err
	}
	if startErr := xrayUpdateStart(s); startErr != nil {
		logger.Errorf("xray update: %s did not start, rolling back: %v", version, startErr)
		if rbErr := restoreXrayBinary(target, backup); rbErr != nil {
			return fmt.Errorf("xray %s failed to start (%v) and rollback failed: %w", version, startErr, rbErr)
		}
		if again := xrayUpdateStart(s); again != nil {
			return fmt.Errorf("xray %s failed to start (%v); rolled back the binary but the previous version did not start either: %w", version, startErr, again)
		}
		return fmt.Errorf("xray %s failed to start and was rolled back to the previous version: %w%s", version, startErr, sandboxHint(startErr))
	}
	logger.Infof("xray updated to %s (previous binary kept as %s)", version, backup)
	return nil
}
