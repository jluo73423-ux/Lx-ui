package xray

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestTierOfXrayVersion(t *testing.T) {
	cases := map[string]XrayVersionTier{
		RecommendedXrayVersion: XrayTierRecommended,
		"26.10.21":             XrayTierVerified,
		"26.6.27":              XrayTierRecommended,
		"26.10.10":             XrayTierUnsupported, // withdrawn upstream
		"v26.10.17":            XrayTierUnsupported, // withdrawn upstream
		"v26.9.8":              XrayTierUnsupported, // withdrawn upstream
		"v26.9.30":             XrayTierVerified,
		"v26.7.11":             XrayTierVerified,
		"v26.8.1":              XrayTierExperimental,
		"v26.11.2":             XrayTierExperimental,
		"v27.1.1":              XrayTierExperimental,
		"v26.6.22":             XrayTierUnsupported,
		"v25.12.31":            XrayTierUnsupported,
		"v26.11.2-beta":        XrayTierUnsupported,
		"latest":               XrayTierUnsupported,
		"":                     XrayTierUnsupported,
	}
	for tag, want := range cases {
		if got := TierOfXrayVersion(tag); got != want {
			t.Errorf("TierOfXrayVersion(%q) = %s, want %s", tag, got, want)
		}
	}
}

func TestXrayVersionAllowed(t *testing.T) {
	if !XrayVersionAllowed(RecommendedXrayVersion, false) || !XrayVersionAllowed("v26.7.11", false) {
		t.Fatal("recommended and verified versions must always be installable")
	}
	if XrayVersionAllowed("v26.11.2", false) {
		t.Fatal("experimental versions must be refused without opt-in")
	}
	if !XrayVersionAllowed("v26.11.2", true) {
		t.Fatal("experimental versions are installable after opt-in")
	}
	if XrayVersionAllowed("v26.6.22", true) {
		t.Fatal("versions below the minimum are never installable")
	}
}

// The recommended core is a verified pin chosen for client compatibility,
// not necessarily the newest one (newer cores gate REALITY clients).
func TestVerifiedVersionsAreOrderedAndContainRecommended(t *testing.T) {
	found := false
	for i, v := range VerifiedXrayVersions {
		if v == RecommendedXrayVersion {
			found = true
		}
		if CompareXrayVersions(v, MinXrayVersion) < 0 {
			t.Errorf("verified %s is below the minimum %s", v, MinXrayVersion)
		}
		if i > 0 && CompareXrayVersions(VerifiedXrayVersions[i-1], v) <= 0 {
			t.Errorf("VerifiedXrayVersions not newest-first at %d", i)
		}
	}
	if !found {
		t.Errorf("recommended %s must be in VerifiedXrayVersions", RecommendedXrayVersion)
	}
}

// The bundled binary (release archives, Docker image) must be the recommended
// version, otherwise fresh installs would start on a different tier.
func TestRecommendedXrayVersionMatchesPins(t *testing.T) {
	root := filepath.Join("..", "..")
	// Either a literal download URL or the workflow-level XRAY_VERSION variable.
	re := regexp.MustCompile(`Xray-core/releases/download/(v[0-9]+\.[0-9]+\.[0-9]+)/|XRAY_VERSION:\s*"?(v[0-9]+\.[0-9]+\.[0-9]+)"?`)
	for _, rel := range []string{".github/workflows/release.yml", "DockerInit.sh"} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		m := re.FindAllSubmatch(data, -1)
		if len(m) == 0 {
			t.Fatalf("%s: no Xray version pin found", rel)
		}
		for _, sm := range m {
			got := string(sm[1]) + string(sm[2])
			if got != RecommendedXrayVersion {
				t.Errorf("%s pins Xray %s, want RecommendedXrayVersion %s", rel, got, RecommendedXrayVersion)
			}
		}
	}
}

func TestRealityGateOf(t *testing.T) {
	cases := map[string]RealityGate{
		"v26.3.27":  RealityGateNone,
		"v26.6.27":  RealityGateNone,
		"v26.7.10":  RealityGateNone,
		"v26.7.11":  RealityGateMinVer,
		"26.7.28":   RealityGateMinVer,
		"v26.9.7":   RealityGateMinVer,
		"v26.9.8":   RealityGateMLKEM,
		"v26.9.30":  RealityGateMLKEM,
		"v26.10.10": RealityGateMLKEM,
		"v26.10.21": RealityGateMLKEM,
		"nightly":   RealityGateNone,
	}
	for tag, want := range cases {
		if got := RealityGateOf(tag); got != want {
			t.Errorf("RealityGateOf(%q) = %q, want %q", tag, got, want)
		}
	}
}

// update.sh keeps the operator's core only when it is a supported release; its
// list must match the panel's.
func TestUpdateScriptSupportedXrayListMatches(t *testing.T) {
	b, err := os.ReadFile("../../update.sh")
	if err != nil {
		t.Fatal(err)
	}
	want := " "
	for _, v := range VerifiedXrayVersions {
		want += strings.TrimPrefix(v, "v") + " "
	}
	if !strings.Contains(string(b), `supported_xray="`+want+`"`) {
		t.Fatalf("update.sh supported_xray must be %q", want)
	}
}
