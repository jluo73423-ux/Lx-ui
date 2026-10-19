package runtime

import (
	"context"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
)

func TestUpdateTuicInboundResyncsBridgeOnTagRename(t *testing.T) {
	resyncs := 0
	local := NewLocal(LocalDeps{SetNeedRestart: func() { resyncs++ }})
	oldInbound := &model.Inbound{Id: 940001, Tag: "old-tuic-tag", Protocol: model.TUIC, Enable: true, Settings: `{"clients":[]}`}
	newInbound := *oldInbound
	newInbound.Tag = "new-tuic-tag"

	if err := local.updateTuicInbound(context.Background(), oldInbound, &newInbound); err != nil {
		t.Fatalf("updateTuicInbound: %v", err)
	}
	if resyncs != 1 {
		t.Fatalf("Xray bridge resync count = %d, want 1", resyncs)
	}
}

func TestUpdateTuicInboundDoesNotResyncForProfileOnlyChange(t *testing.T) {
	resyncs := 0
	local := NewLocal(LocalDeps{SetNeedRestart: func() { resyncs++ }})
	oldInbound := &model.Inbound{Id: 940002, Tag: "tuic-tag", Protocol: model.TUIC, Enable: true, Settings: `{"clients":[],"congestion_control":"bbr"}`}
	newInbound := *oldInbound
	newInbound.Settings = `{"clients":[],"congestion_control":"cubic","log_level":"debug"}`

	if err := local.updateTuicInbound(context.Background(), oldInbound, &newInbound); err != nil {
		t.Fatalf("updateTuicInbound: %v", err)
	}
	if resyncs != 0 {
		t.Fatalf("profile-only update triggered %d Xray restarts, want 0", resyncs)
	}
}

func TestUpdateTuicInboundResyncsWhenAClientIsAdded(t *testing.T) {
	resyncs := 0
	local := NewLocal(LocalDeps{SetNeedRestart: func() { resyncs++ }})
	const id = 940003
	tuic.RecordRelayAccounts(id, []string{"a@example.test"})
	oldInbound := &model.Inbound{Id: id, Tag: "tuic-acc", Protocol: model.TUIC, Enable: true,
		Settings: `{"clients":[{"email":"a@example.test"}]}`}
	newInbound := *oldInbound
	newInbound.Settings = `{"clients":[{"email":"a@example.test"},{"email":"b@example.test"}]}`
	if err := local.updateTuicInbound(context.Background(), oldInbound, &newInbound); err != nil {
		t.Fatalf("updateTuicInbound: %v", err)
	}
	if resyncs != 1 {
		t.Fatalf("adding a TUIC client triggered %d Xray resyncs, want 1 (relay account missing, #6874)", resyncs)
	}
}

func TestUpdateTuicInboundDoesNotResyncWhenAccountsAreCovered(t *testing.T) {
	resyncs := 0
	local := NewLocal(LocalDeps{SetNeedRestart: func() { resyncs++ }})
	const id = 940004
	tuic.RecordRelayAccounts(id, []string{"a@example.test", "b@example.test"})
	oldInbound := &model.Inbound{Id: id, Tag: "tuic-acc2", Protocol: model.TUIC, Enable: true,
		Settings: `{"clients":[{"email":"a@example.test","enable":true},{"email":"b@example.test","enable":true}]}`}
	newInbound := *oldInbound
	// b disabled: the runtime payload drops it, the relay account stays.
	newInbound.Settings = `{"clients":[{"email":"a@example.test","enable":true}]}`
	if err := local.updateTuicInbound(context.Background(), oldInbound, &newInbound); err != nil {
		t.Fatalf("updateTuicInbound: %v", err)
	}
	if resyncs != 0 {
		t.Fatalf("disabling a TUIC client triggered %d Xray resyncs, want 0", resyncs)
	}
}
