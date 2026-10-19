package service

import "testing"

func TestNodeSyncCadenceSettings(t *testing.T) {
	initTrafficTestDB(t)
	s := &SettingService{}
	if got := s.GetNodeHeartbeatInterval(); got != NodeSyncIntervalDefault {
		t.Fatalf("default heartbeat = %d, want %d", got, NodeSyncIntervalDefault)
	}
	if got := s.GetNodeTrafficInterval(); got != NodeSyncIntervalDefault {
		t.Fatalf("default traffic = %d, want %d", got, NodeSyncIntervalDefault)
	}
	if got := s.GetNodeSyncJitter(); got != 0 {
		t.Fatalf("default jitter = %d, want 0", got)
	}
	cases := []struct {
		key  string
		val  string
		get  func() int
		want int
	}{
		{"nodeTrafficInterval", "30", s.GetNodeTrafficInterval, 30},
		{"nodeTrafficInterval", "1", s.GetNodeTrafficInterval, NodeSyncIntervalMin},
		{"nodeTrafficInterval", "100000", s.GetNodeTrafficInterval, NodeSyncIntervalMax},
		{"nodeTrafficInterval", "junk", s.GetNodeTrafficInterval, NodeSyncIntervalDefault},
		{"nodeHeartbeatInterval", "15", s.GetNodeHeartbeatInterval, 15},
		{"nodeSyncJitter", "3", s.GetNodeSyncJitter, 3},
		{"nodeSyncJitter", "-4", s.GetNodeSyncJitter, 0},
		{"nodeSyncJitter", "999", s.GetNodeSyncJitter, NodeSyncJitterMax},
	}
	for _, c := range cases {
		if err := s.setString(c.key, c.val); err != nil {
			t.Fatal(err)
		}
		if got := c.get(); got != c.want {
			t.Errorf("%s=%q: got %d, want %d", c.key, c.val, got, c.want)
		}
	}
}
