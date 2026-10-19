package service

import (
	"fmt"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// Regression tests for #6870 (multi-node client desync): a node reporting
// enable=false for a client that, by the master's own accounting and by the
// node's own counters, still has quota and time left must not latch the
// master row to disabled. Upstream latched it whenever the node's limits
// matched the master's, after which the master's lifecycle re-enabled the
// client and re-pushed it -- the client flipped on every sync.

func syncNodeStructural(t *testing.T, svc *InboundService, nodeID int, tag string, stats ...xray.ClientTraffic) bool {
	t.Helper()
	snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{{Tag: tag, ClientStats: stats}}}
	changed, err := svc.setRemoteTrafficLocked(nodeID, snap, false, false)
	if err != nil {
		t.Fatalf("setRemoteTrafficLocked node %d: %v", nodeID, err)
	}
	return changed
}

// The data reported in #6870 (node 8): local up+down far below total, deadline
// ahead, limits identical to the master's, but enable=0.
func TestNodeDisableWithoutEvidenceDoesNotLatch(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 8, "n8-in", 41008)
	svc := &InboundService{}

	const email = "desync"
	const total = int64(100 << 30)
	expiry := lateAbs

	syncNode(t, svc, 8, "n8-in", xray.ClientTraffic{
		Email: email, Up: 1 << 20, Down: 2 << 20, Total: total, ExpiryTime: expiry, Enable: true,
	})
	for tick := 1; tick <= 5; tick++ {
		changed := syncNodeStructural(t, svc, 8, "n8-in", xray.ClientTraffic{
			Email: email, Up: 1<<20 + int64(tick), Down: 2 << 20, Total: total, ExpiryTime: expiry, Enable: false,
		})
		got := readTraffic(t, db, email)
		if !got.Enable {
			t.Fatalf("tick %d: node disable without evidence latched the master row (#6870)", tick)
		}
		if changed {
			t.Fatalf("tick %d: an unconfirmed node disable must not count as a structural change (re-push storm)", tick)
		}
	}
}

// The race behind #6870: a global-traffic push computed before a master-side
// traffic reset lands on the node after it; the node disables the client on
// those stale global numbers and reports enable=false with matching limits
// while its own counters are low. The master, whose counters were just reset,
// must keep the client enabled.
func TestNodeDisableFromStaleGlobalAfterMasterResetDoesNotLatch(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	createNodeInbound(t, db, 2, "n2-in", 41002)
	svc := &InboundService{}

	const email = "multi"
	const total = int64(1000)
	expiry := lateAbs

	// Usage spread over two nodes brings the global total close to the quota.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 0, Down: 0, Total: total, ExpiryTime: expiry, Enable: true})
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 0, Down: 0, Total: total, ExpiryTime: expiry, Enable: true})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 300, Down: 200, Total: total, ExpiryTime: expiry, Enable: true})
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 200, Down: 250, Total: total, ExpiryTime: expiry, Enable: true})
	if got := readTraffic(t, db, email); got.Up+got.Down != 950 || !got.Enable {
		t.Fatalf("setup: master usage %d enable %v, want 950 true", got.Up+got.Down, got.Enable)
	}

	// Admin resets the client's traffic on the master.
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"up": 0, "down": 0}).Error; err != nil {
		t.Fatal(err)
	}

	// Node 2 still holds the pre-reset global (950 of 1000) plus a little new
	// local usage: its global-quota check trips and it reports enable=false.
	syncNode(t, svc, 2, "n2-in", xray.ClientTraffic{Email: email, Up: 230, Down: 250, Total: total, ExpiryTime: expiry, Enable: false})
	if got := readTraffic(t, db, email); !got.Enable {
		t.Fatal("a node disable based on pre-reset global usage latched the freshly reset client (#6870)")
	}
}

// Genuine verdicts still latch (#4917): the node's own counters reach the
// quota even though the master attributed less (usage before its baseline).
func TestNodeDisableWithOwnCountersOverQuotaLatches(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 3, "n3-in", 41003)
	svc := &InboundService{}

	const email = "genuine"
	syncNode(t, svc, 3, "n3-in", xray.ClientTraffic{Email: email, Up: 400, Down: 400, Total: 1000, ExpiryTime: lateAbs, Enable: true})
	syncNode(t, svc, 3, "n3-in", xray.ClientTraffic{Email: email, Up: 500, Down: 500, Total: 1000, ExpiryTime: lateAbs, Enable: false})
	if got := readTraffic(t, db, email); got.Enable {
		t.Fatal("node disable backed by its own counters reaching the quota must latch")
	}
}

// A node disable for a client whose master deadline has passed latches even
// when the node's counters are low.
func TestNodeDisableForExpiredClientLatches(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 4, "n4-in", 41004)
	svc := &InboundService{}

	const email = "expired"
	syncNode(t, svc, 4, "n4-in", xray.ClientTraffic{Email: email, Up: 1, Down: 1, Total: 1000, ExpiryTime: earlyAbs, Enable: true})
	syncNode(t, svc, 4, "n4-in", xray.ClientTraffic{Email: email, Up: 2, Down: 1, Total: 1000, ExpiryTime: earlyAbs, Enable: false})
	if got := readTraffic(t, db, email); got.Enable {
		t.Fatal("node disable for an expired client must latch")
	}
}

func TestNodeDisableIsStaleUsesNodeEvidence(t *testing.T) {
	master := &xray.ClientTraffic{ExpiryTime: lateAbs, Total: 1000, Up: 100, Down: 100}
	if !nodeDisableIsStale(master, xray.ClientTraffic{ExpiryTime: lateAbs, Total: 1000, Up: 10, Down: 10}, 0, 0, 0) {
		t.Fatal("no evidence: must be stale")
	}
	if nodeDisableIsStale(master, xray.ClientTraffic{ExpiryTime: lateAbs, Total: 1000, Up: 600, Down: 400}, 0, 0, 0) {
		t.Fatal("node counters at the quota: verdict is genuine")
	}
}

// With the node's settings blob showing the client disabled, the master keeps
// its row enabled and flags the node for a re-push instead of flipping its own
// state (the self-healing direction of #6870).
func TestNodeDisableWithoutEvidenceRepushesInsteadOfFlipping(t *testing.T) {
	db := initTrafficTestDB(t)
	const nodeID = 9
	if err := db.Create(&model.Node{Id: nodeID, Name: "node9", Address: "10.0.0.9", Port: 2053, ApiToken: "t", Guid: "node9-guid"}).Error; err != nil {
		t.Fatal(err)
	}
	const email = "heal"
	createNodeInboundWithClient(t, db, nodeID, "n9-in", 41009, email)
	svc := &InboundService{}

	enabled := `{"clients":[{"email":"heal","id":"a0000000-0000-0000-0000-000000000009","enable":true,"expiryTime":%d}]}`
	disabled := `{"clients":[{"email":"heal","id":"a0000000-0000-0000-0000-000000000009","enable":false,"expiryTime":%d}]}`
	enabled = fmt.Sprintf(enabled, lateAbs)
	disabled = fmt.Sprintf(disabled, lateAbs)
	syncNodeWithSettings(t, svc, nodeID, "n9-in", enabled, xray.ClientTraffic{
		Email: email, Up: 10, Down: 10, Total: 1 << 30, ExpiryTime: lateAbs, Enable: true,
	})
	if err := db.Model(&model.Node{}).Where("id = ?", nodeID).Update("config_dirty", false).Error; err != nil {
		t.Fatal(err)
	}
	syncNodeWithSettings(t, svc, nodeID, "n9-in", disabled, xray.ClientTraffic{
		Email: email, Up: 11, Down: 10, Total: 1 << 30, ExpiryTime: lateAbs, Enable: false,
	})
	if got := readTraffic(t, db, email); !got.Enable {
		t.Fatal("master row latched the unconfirmed node disable")
	}
	var node model.Node
	if err := db.First(&node, nodeID).Error; err != nil {
		t.Fatal(err)
	}
	if !node.ConfigDirty {
		t.Fatal("node must be marked dirty so the master re-pushes the enabled client")
	}
}
