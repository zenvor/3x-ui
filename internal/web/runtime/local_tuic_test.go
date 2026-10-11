package runtime

import (
	"context"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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

func TestUpdateTuicInboundResyncsWhenSnapshotsMayAlreadyBeCurrent(t *testing.T) {
	for _, settings := range []string{
		`{"clients":[],"congestion_control":"bbr"}`,
		`{"clients":[],"congestion_control":"cubic","log_level":"debug"}`,
	} {
		t.Run(settings, func(t *testing.T) {
			resyncs := 0
			local := NewLocal(LocalDeps{SetNeedRestart: func() { resyncs++ }})
			oldInbound := &model.Inbound{Id: 940002, Tag: "tuic-tag", Protocol: model.TUIC, Enable: true, Settings: `{"clients":[],"congestion_control":"bbr"}`}
			newInbound := *oldInbound
			newInbound.Settings = settings

			if err := local.updateTuicInbound(context.Background(), oldInbound, &newInbound); err != nil {
				t.Fatalf("updateTuicInbound: %v", err)
			}
			if resyncs != 1 {
				t.Fatalf("runtime apply triggered %d Xray resyncs, want 1", resyncs)
			}
		})
	}
}
