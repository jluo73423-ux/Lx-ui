package service

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// fakeXrayScript emulates the two Xray commands the updater runs.
func fakeXrayScript(version string, testExit int, testMsg string) string {
	return "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  version) echo \"Xray " + version + " (Xray, Penetrates Everything.) Custom (go1.27 linux/amd64)\" ;;\n" +
		"  run)\n" +
		"    [ \"$2\" = -test ] && [ \"$3\" = -c ] && [ -f \"$4\" ] || { echo bad args >&2; exit 2; }\n" +
		"    echo \"" + testMsg + "\" >&2\n" +
		"    exit " + strconv.Itoa(testExit) + " ;;\n" +
		"  *) exit 1 ;;\n" +
		"esac\n"
}

func writeXrayZip(t *testing.T, script string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "Xray-linux-64.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	h := &zip.FileHeader{Name: "xray", Method: zip.Deflate}
	h.SetMode(0o755)
	w, err := zw.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(script)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

type updateHarness struct {
	target       string
	stops        int
	starts       int
	startResults []error
}

func newUpdateHarness(t *testing.T, oldBinary string) *updateHarness {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake xray")
	}
	bin := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", bin)
	h := &updateHarness{target: xray.GetBinaryPath()}
	if oldBinary != "" {
		if err := os.WriteFile(h.target, []byte(oldBinary), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oldStop, oldStart, oldCfg := xrayUpdateStop, xrayUpdateStart, xrayUpdateConfig
	t.Cleanup(func() { xrayUpdateStop, xrayUpdateStart, xrayUpdateConfig = oldStop, oldStart, oldCfg })
	xrayUpdateStop = func(*ServerService) error { h.stops++; return nil }
	xrayUpdateStart = func(*ServerService) error {
		h.starts++
		if len(h.startResults) == 0 {
			return nil
		}
		r := h.startResults[0]
		h.startResults = h.startResults[1:]
		return r
	}
	xrayUpdateConfig = func(*ServerService) ([]byte, error) { return []byte(`{"inbounds":[]}`), nil }
	return h
}

func (h *updateHarness) installed(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(h.target)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func assertNoStagedLeftovers(t *testing.T, dir string) {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".xray-") {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
}

func TestInstallXrayArchiveSwapsValidatedBinaryAndKeepsBackup(t *testing.T) {
	h := newUpdateHarness(t, "OLD")
	script := fakeXrayScript("26.10.10", 0, "Configuration OK.")
	if err := (&ServerService{}).installXrayArchive(writeXrayZip(t, script), "v26.10.10"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if got := h.installed(t); got != script {
		t.Fatalf("new binary not installed: %q", got)
	}
	if b, err := os.ReadFile(h.target + xrayBackupSuffix); err != nil || string(b) != "OLD" {
		t.Fatalf("backup = %q, %v; want OLD", b, err)
	}
	if h.stops != 1 || h.starts != 1 {
		t.Fatalf("stops=%d starts=%d, want 1/1", h.stops, h.starts)
	}
	assertNoStagedLeftovers(t, filepath.Dir(h.target))
}

func TestInstallXrayArchiveRejectsVersionMismatchWithoutStopping(t *testing.T) {
	h := newUpdateHarness(t, "OLD")
	err := (&ServerService{}).installXrayArchive(writeXrayZip(t, fakeXrayScript("26.11.2", 0, "ok")), "v26.10.10")
	if err == nil || !strings.Contains(err.Error(), "expected 26.10.10") {
		t.Fatalf("err = %v, want version mismatch", err)
	}
	if h.installed(t) != "OLD" || h.stops != 0 || h.starts != 0 {
		t.Fatalf("running xray was touched: binary=%q stops=%d starts=%d", h.installed(t), h.stops, h.starts)
	}
	assertNoStagedLeftovers(t, filepath.Dir(h.target))
}

// The #6871 failure mode: the new core cannot be created in this environment.
// The config test reproduces it before anything is stopped.
func TestInstallXrayArchiveRejectsBinaryFailingConfigTest(t *testing.T) {
	h := newUpdateHarness(t, "OLD")
	script := fakeXrayScript("26.10.10", 23, "main: failed to create server > operation not permitted")
	err := (&ServerService{}).installXrayArchive(writeXrayZip(t, script), "v26.10.10")
	if err == nil || !strings.Contains(err.Error(), "operation not permitted") || !strings.Contains(err.Error(), "keeps running") || !strings.Contains(err.Error(), "sandbox") {
		t.Fatalf("err = %v, want config-test failure", err)
	}
	if h.installed(t) != "OLD" || h.stops != 0 || h.starts != 0 {
		t.Fatalf("running xray was touched: binary=%q stops=%d starts=%d", h.installed(t), h.stops, h.starts)
	}
	if _, err := os.Stat(h.target + xrayBackupSuffix); !os.IsNotExist(err) {
		t.Fatal("no backup should be created when validation fails")
	}
}

func TestInstallXrayArchiveRollsBackWhenNewBinaryWontStart(t *testing.T) {
	h := newUpdateHarness(t, "OLD")
	h.startResults = []error{errors.New("xray exited after start: exit status 23"), nil}
	err := (&ServerService{}).installXrayArchive(writeXrayZip(t, fakeXrayScript("26.10.10", 0, "ok")), "v26.10.10")
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("err = %v, want rollback", err)
	}
	if got := h.installed(t); got != "OLD" {
		t.Fatalf("binary after rollback = %q, want OLD", got)
	}
	if h.starts != 2 {
		t.Fatalf("starts = %d, want 2 (new, then previous)", h.starts)
	}
}

func TestInstallXrayArchiveReportsFailedRollbackWithoutPreviousBinary(t *testing.T) {
	h := newUpdateHarness(t, "")
	h.startResults = []error{errors.New("boom")}
	err := (&ServerService{}).installXrayArchive(writeXrayZip(t, fakeXrayScript("26.10.10", 0, "ok")), "v26.10.10")
	if err == nil || !strings.Contains(err.Error(), "rollback failed") {
		t.Fatalf("err = %v, want rollback failure", err)
	}
}

func TestInstallXrayArchiveWithoutConfigStillChecksVersion(t *testing.T) {
	h := newUpdateHarness(t, "OLD")
	xrayUpdateConfig = func(*ServerService) ([]byte, error) { return nil, errors.New("db down") }
	// Config test would fail, but it is skipped when no config can be built.
	script := fakeXrayScript("26.10.10", 23, "would fail")
	if err := (&ServerService{}).installXrayArchive(writeXrayZip(t, script), "v26.10.10"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if h.installed(t) != script {
		t.Fatal("binary not installed")
	}
}

func TestUpdateXrayRefusesExperimentalWithoutOptIn(t *testing.T) {
	initTrafficTestDB(t)
	err := (&ServerService{}).UpdateXray("v99.1.1")
	if err == nil || !strings.Contains(err.Error(), "experimental") {
		t.Fatalf("err = %v, want experimental refusal", err)
	}
	err = (&ServerService{}).UpdateXray("v26.1.18")
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("err = %v, want unsupported refusal", err)
	}
}

func TestBuildXrayVersionInfos(t *testing.T) {
	fetched := []string{"v26.11.2", "v26.10.21", "v26.10.10", "v26.9.30", "v26.8.1"}
	infos := buildXrayVersionInfos(fetched, false)
	var tags []string
	for _, i := range infos {
		tags = append(tags, i.Version+":"+string(i.Tier))
	}
	want := "v26.10.21:recommended v26.9.30:verified v26.7.28:verified v26.7.11:verified v26.6.27:verified"
	if got := strings.Join(tags, " "); got != want {
		t.Fatalf("infos = %s\nwant    %s", got, want)
	}
	gates := map[string]xray.RealityGate{}
	for _, i := range infos {
		gates[i.Version] = i.RealityGate
	}
	if gates["v26.10.21"] != xray.RealityGateMLKEM || gates["v26.7.28"] != xray.RealityGateMinVer || gates["v26.6.27"] != xray.RealityGateNone {
		t.Fatalf("reality gates = %v", gates)
	}
	infos = buildXrayVersionInfos(fetched, true)
	if infos[0].Version != "v26.11.2" || infos[0].Tier != xray.XrayTierExperimental {
		t.Fatalf("with opt-in the newest experimental release must lead: %+v", infos[0])
	}
	if len(buildXrayVersionInfos(nil, false)) != 1+len(xray.VerifiedXrayVersions) {
		t.Fatal("pinned versions must be offered even without a GitHub list")
	}
}

// Opt-in check against a real Xray binary: LXUI_REAL_XRAY=/path/to/xray
// LXUI_REAL_XRAY_VERSION=v26.10.10 go test -run RealXray ./internal/web/service
func TestValidateRealXrayBinary(t *testing.T) {
	bin := os.Getenv("LXUI_REAL_XRAY")
	if bin == "" {
		t.Skip("LXUI_REAL_XRAY not set")
	}
	version := os.Getenv("LXUI_REAL_XRAY_VERSION")
	t.Setenv("XUI_BIN_FOLDER", filepath.Dir(bin))
	cfg := []byte(`{"log":{"loglevel":"warning"},"inbounds":[{"tag":"api","listen":"127.0.0.1","port":62789,"protocol":"dokodemo-door","settings":{"address":"127.0.0.1"}}],"outbounds":[{"protocol":"freedom","tag":"direct"}]}`)
	if err := validateXrayBinary(t.Context(), bin, version, cfg); err != nil {
		t.Fatalf("validate %s: %v", bin, err)
	}
	if err := validateXrayBinary(t.Context(), bin, "v1.0.0", cfg); err == nil {
		t.Fatal("version mismatch not detected on the real binary")
	}
	if err := validateXrayBinary(t.Context(), bin, version, []byte(`{"inbounds":[{"protocol":"nope"}]}`)); err == nil {
		t.Fatal("broken config not detected on the real binary")
	}
}

// Real installs run with the default relative bin folder ("bin", resolved
// against WorkingDirectory=/usr/local/lx-ui). The config test used to run the
// staged "bin/.xray-new-*" with cmd.Dir=bin, i.e. bin/bin/.xray-new-*, and
// every panel Xray update failed with "fork/exec ...: no such file or
// directory" (seen on a live install).
func TestInstallXrayArchiveWithRelativeBinFolder(t *testing.T) {
	h := newUpdateHarness(t, "")
	work := t.TempDir()
	t.Chdir(work)
	if err := os.MkdirAll("bin", 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XUI_BIN_FOLDER", "bin")
	h.target = xray.GetBinaryPath()
	if filepath.IsAbs(h.target) {
		t.Fatalf("precondition: target %q should be relative", h.target)
	}
	if err := os.WriteFile(h.target, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := fakeXrayScript("26.10.10", 0, "Configuration OK.")
	if err := (&ServerService{}).installXrayArchive(writeXrayZip(t, script), "v26.10.10"); err != nil {
		t.Fatalf("install with relative bin folder: %v", err)
	}
	if got := h.installed(t); got != script {
		t.Fatalf("new binary not installed: %q", got)
	}
	assertNoStagedLeftovers(t, filepath.Join(work, "bin"))
}
