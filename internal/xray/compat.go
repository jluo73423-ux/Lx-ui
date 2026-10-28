package xray

import (
	"strconv"
	"strings"
)

// Xray-core version policy (Lx-ui).
//
// The panel generates configs with the xray-core Go module pinned in go.mod
// and ships one Xray binary per release. Switching the external binary to an
// arbitrary release can break in ways the panel cannot see (config semantics,
// new kernel/sandbox requirements; see #6871), so every installable release is
// put in one of three tiers:
//
//   - recommended: the version this build ships and is tested against
//     (release.yml, DockerInit.sh and the go.mod module all match it).
//   - verified: versions upstream shipped with earlier panel releases at or
//     above the panel's minimum. Known to work with this config generator
//     family, but not re-tested against this exact build.
//   - experimental: anything else at or above the minimum, typically newer
//     than recommended. Hidden and refused unless the operator opts in
//     (setting xrayAllowExperimental).
//
// Keep RecommendedXrayVersion in sync with .github/workflows/release.yml and
// DockerInit.sh; TestRecommendedXrayVersionMatchesPins enforces that.
const (
	RecommendedXrayVersion = "v26.6.27"
	// MinXrayVersion is the oldest release the panel can drive at all.
	MinXrayVersion = "v26.6.27"
)

// VerifiedXrayVersions lists, newest first, the release pins upstream shipped
// since MinXrayVersion (from the history of release.yml).
var VerifiedXrayVersions = []string{
	"v26.10.21",
	"v26.9.30",
	"v26.7.28",
	"v26.7.11",
	"v26.6.27",
}

// WithdrawnXrayVersions were pulled upstream (root-CA security issue, see the
// v26.10.21 release notes); the panel refuses to install them.
var WithdrawnXrayVersions = []string{"v26.9.8", "v26.10.10", "v26.10.17"}

// XrayVersionTier names a version's place in the policy above.
type XrayVersionTier string

const (
	XrayTierRecommended  XrayVersionTier = "recommended"
	XrayTierVerified     XrayVersionTier = "verified"
	XrayTierExperimental XrayVersionTier = "experimental"
	// XrayTierUnsupported is below MinXrayVersion or not a release tag.
	XrayTierUnsupported XrayVersionTier = "unsupported"
)

// ParseXrayVersion parses "vX.Y.Z" or "X.Y.Z" (no pre-release suffix).
func ParseXrayVersion(tag string) (major, minor, patch int, ok bool) {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(tag), "v"), ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return 0, 0, 0, false
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], true
}

// CompareXrayVersions returns -1, 0 or 1. Unparseable tags sort first.
func CompareXrayVersions(a, b string) int {
	am, an, ap, aok := ParseXrayVersion(a)
	bm, bn, bp, bok := ParseXrayVersion(b)
	switch {
	case !aok && !bok:
		return 0
	case !aok:
		return -1
	case !bok:
		return 1
	}
	for _, d := range [][2]int{{am, bm}, {an, bn}, {ap, bp}} {
		if d[0] != d[1] {
			if d[0] < d[1] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// normalizeTag returns "vX.Y.Z" for a parseable tag.
func normalizeTag(tag string) string {
	m, n, p, ok := ParseXrayVersion(tag)
	if !ok {
		return tag
	}
	return "v" + strconv.Itoa(m) + "." + strconv.Itoa(n) + "." + strconv.Itoa(p)
}

// TierOfXrayVersion classifies a release tag.
func TierOfXrayVersion(tag string) XrayVersionTier {
	if _, _, _, ok := ParseXrayVersion(tag); !ok {
		return XrayTierUnsupported
	}
	if CompareXrayVersions(tag, MinXrayVersion) < 0 {
		return XrayTierUnsupported
	}
	t := normalizeTag(tag)
	for _, v := range WithdrawnXrayVersions {
		if t == v {
			return XrayTierUnsupported
		}
	}
	if t == RecommendedXrayVersion {
		return XrayTierRecommended
	}
	for _, v := range VerifiedXrayVersions {
		if t == v {
			return XrayTierVerified
		}
	}
	return XrayTierExperimental
}

// XrayVersionAllowed reports whether a tag may be installed under the policy.
func XrayVersionAllowed(tag string, allowExperimental bool) bool {
	switch TierOfXrayVersion(tag) {
	case XrayTierRecommended, XrayTierVerified:
		return true
	case XrayTierExperimental:
		return allowExperimental
	default:
		return false
	}
}

// RealityGate names the REALITY server-side ClientHello gate a core release
// enforces. These gates silently divert a client to the REALITY target (the
// TLS handshake "succeeds" against the real site, nothing is proxied and the
// server logs nothing), which is why REALITY nodes can look online yet carry
// no traffic for third-party clients. Verified against Xray-core sources and
// with real handshakes (see docs/LX_CHANGES.md, "REALITY client gates").
type RealityGate string

const (
	// RealityGateNone: no gate (cores before v26.7.11).
	RealityGateNone RealityGate = "none"
	// RealityGateMinVer: v26.7.11 .. v26.9.7 default an empty minClientVer to
	// 26.3.27, rejecting clients that report an older version (Shadowrocket,
	// Mihomo/Clash, sing-box ...). An explicit minClientVer overrides it.
	RealityGateMinVer RealityGate = "minver"
	// RealityGateMLKEM: v26.9.8 and later (github.com/xtls/reality 8cdf7bf9c7f0)
	// reject every ClientHello without an X25519MLKEM768 key share listed before
	// X25519. Not configurable. Shadowrocket, sing-box, Mihomo raw vless:// links
	// and uTLS fingerprints such as ios/edge/qq/random are affected; current
	// Xray-core clients with fp=chrome/firefox/safari are not.
	RealityGateMLKEM RealityGate = "mlkem"
)

const (
	realityMinVerSince = "v26.7.11"
	realityMLKEMSince  = "v26.9.8"
)

// RealityGateOf returns the REALITY gate of a release tag ("none" for
// unparseable tags).
func RealityGateOf(tag string) RealityGate {
	if _, _, _, ok := ParseXrayVersion(tag); !ok {
		return RealityGateNone
	}
	switch {
	case CompareXrayVersions(tag, realityMLKEMSince) >= 0:
		return RealityGateMLKEM
	case CompareXrayVersions(tag, realityMinVerSince) >= 0:
		return RealityGateMinVer
	default:
		return RealityGateNone
	}
}
