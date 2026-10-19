package job

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestGlobalPayloadDigestIgnoresOrderAndUnstoredFields(t *testing.T) {
	a := []*xray.ClientTraffic{{Email: "a", Up: 1, Down: 2, LastOnline: 10}, {Email: "b", Up: 3, Down: 4}}
	b := []*xray.ClientTraffic{{Email: "b", Up: 3, Down: 4, LastOnline: 99}, nil, {Email: "a", Up: 1, Down: 2}}
	if globalPayloadDigest("m", a) != globalPayloadDigest("m", b) {
		t.Fatal("digest must ignore row order, nil rows and lastOnline")
	}
	c := []*xray.ClientTraffic{{Email: "a", Up: 1, Down: 3}, {Email: "b", Up: 3, Down: 4}}
	if globalPayloadDigest("m", a) == globalPayloadDigest("m", c) {
		t.Fatal("a usage change must change the digest")
	}
	if globalPayloadDigest("m", a) == globalPayloadDigest("other", a) {
		t.Fatal("the master guid is part of what the node stores")
	}
	d := []*xray.ClientTraffic{{Email: "a", Up: 1, Down: 2}}
	if globalPayloadDigest("m", a) == globalPayloadDigest("m", d) {
		t.Fatal("a removed client must change the digest")
	}
}

func TestGlobalPushUnchangedHonoursRefresh(t *testing.T) {
	j := NewNodeTrafficSyncJob()
	rows := []*xray.ClientTraffic{{Email: "a", Up: 1, Down: 2}}
	dg := globalPayloadDigest("m", rows)
	const t0 = int64(1_000_000)
	if j.globalPushUnchanged(1, dg, t0) {
		t.Fatal("first push must not be skipped")
	}
	j.lastGlobalPayload.Store(1, globalPushStamp{digest: dg, at: t0})
	if !j.globalPushUnchanged(1, dg, t0+30) {
		t.Fatal("identical payload 30s later should be skipped")
	}
	if j.globalPushUnchanged(2, dg, t0+30) {
		t.Fatal("stamps are per node")
	}
	other := globalPayloadDigest("m", []*xray.ClientTraffic{{Email: "a", Up: 5, Down: 2}})
	if j.globalPushUnchanged(1, other, t0+30) {
		t.Fatal("changed payload must be pushed")
	}
	refresh := int64(nodeGlobalPushRefresh.Seconds())
	if j.globalPushUnchanged(1, dg, t0+refresh) {
		t.Fatal("unchanged payload must still be refreshed after nodeGlobalPushRefresh")
	}
}
