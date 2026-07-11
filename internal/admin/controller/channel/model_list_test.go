package channel

import (
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
)

func TestResolveChannelModelSyncStatusUsesReturnedUpstreamModel(t *testing.T) {
	row := model.ChannelModel{
		Model:         "gpt-5.1-codex-mini",
		UpstreamModel: "gpt-5.1-codex-mini-2025-11-13",
	}
	syncRows := []model.ChannelModelSyncResult{
		{
			Model:         "gpt-5.1-codex-mini",
			UpstreamModel: "gpt-5.1-codex-mini",
			Returned:      false,
			LastSyncedAt:  100,
		},
		{
			Model:         "gpt-5.1-codex-mini",
			UpstreamModel: "gpt-5.1-codex-mini-2025-11-13",
			Returned:      true,
			LastSyncedAt:  90,
		},
	}

	status, lastSyncedAt := resolveChannelModelSyncStatus(syncRows, row)
	if status != "returned" {
		t.Fatalf("status = %q, want returned", status)
	}
	if lastSyncedAt != 90 {
		t.Fatalf("lastSyncedAt = %d, want 90", lastSyncedAt)
	}
}

func TestResolveChannelModelSyncStatusIgnoresReturnedDisplayModel(t *testing.T) {
	row := model.ChannelModel{
		Model:         "gpt-5.1-codex-mini",
		UpstreamModel: "gpt-5.1-codex-mini-2025-11-13",
	}
	syncRows := []model.ChannelModelSyncResult{
		{
			Model:         "gpt-5.1-codex-mini",
			UpstreamModel: "gpt-5.1-codex-mini",
			Returned:      true,
			LastSyncedAt:  100,
		},
		{
			Model:         "gpt-5.1-codex-mini-2025-11-13",
			UpstreamModel: "gpt-5.1-codex-mini-2025-11-13",
			Returned:      false,
			LastSyncedAt:  90,
		},
	}

	status, lastSyncedAt := resolveChannelModelSyncStatus(syncRows, row)
	if status != "not_returned" {
		t.Fatalf("status = %q, want not_returned", status)
	}
	if lastSyncedAt != 90 {
		t.Fatalf("lastSyncedAt = %d, want 90", lastSyncedAt)
	}
}
