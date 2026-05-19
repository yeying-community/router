package model

import "testing"

func TestNormalizeGroupModelChannelRowsPreserveOrder_DeduplicatesByPrimaryKey(t *testing.T) {
	rows := []GroupModelChannel{
		{
			Group:         " group-a ",
			Model:         "gpt-4.1",
			ChannelId:     " channel-1 ",
			UpstreamModel: "upstream-a",
		},
		{
			Group:         "group-a",
			Model:         "gpt-4.1",
			ChannelId:     "channel-1",
			UpstreamModel: "upstream-b",
		},
		{
			Group:     "group-a",
			Model:     "gpt-4.1",
			ChannelId: "channel-2",
		},
	}

	got := normalizeGroupModelChannelRowsPreserveOrder(rows)
	if len(got) != 2 {
		t.Fatalf("normalizeGroupModelChannelRowsPreserveOrder returned %d rows, want 2", len(got))
	}
	if got[0].Group != "group-a" || got[0].Model != "gpt-4.1" || got[0].ChannelId != "channel-1" {
		t.Fatalf("unexpected first row key: %#v", got[0])
	}
	if got[0].UpstreamModel != "upstream-a" {
		t.Fatalf("unexpected first row upstream model: %q", got[0].UpstreamModel)
	}
	if got[1].UpstreamModel != "gpt-4.1" {
		t.Fatalf("unexpected fallback upstream model: %q", got[1].UpstreamModel)
	}
}

func TestCloneChannelWithPriorityDoesNotMutateOriginal(t *testing.T) {
	originalPriority := int64(0)
	channel := &Channel{
		Id:       "channel-1",
		Priority: &originalPriority,
	}

	cloned := CloneChannelWithPriority(channel, 3)
	if cloned == nil {
		t.Fatalf("expected cloned channel, got nil")
	}
	if cloned == channel {
		t.Fatalf("expected cloned channel to be a different pointer")
	}
	if channel.GetPriority() != 0 {
		t.Fatalf("original priority = %d, want 0", channel.GetPriority())
	}
	if cloned.GetPriority() != 3 {
		t.Fatalf("cloned priority = %d, want 3", cloned.GetPriority())
	}
	if cloned.Priority == channel.Priority {
		t.Fatalf("expected cloned priority pointer to be independent")
	}
}

func TestChannelSelectedModelConfigsSkipsInactiveRows(t *testing.T) {
	channel := &Channel{}
	channel.SetChannelModels([]ChannelModel{
		{
			Model:    "active-model",
			Selected: true,
		},
		{
			Model:    "inactive-model",
			Selected: true,
			Inactive: true,
		},
		{
			Model:    "unselected-model",
			Selected: false,
		},
	})

	got := channelSelectedModels(channel)
	if len(got) != 1 {
		t.Fatalf("channelSelectedModels returned %d rows, want 1", len(got))
	}
	if got[0].Model != "active-model" {
		t.Fatalf("selected model = %q, want active-model", got[0].Model)
	}
}

func TestNormalizeGroupModelRowsPreservesDisabledState(t *testing.T) {
	rows := []GroupModel{
		{
			Group:    " group-a ",
			Model:    " gpt-image-2 ",
			Provider: "openai",
			Enabled:  false,
		},
	}

	got := normalizeGroupModelRows("group-a", rows)
	if len(got) != 1 {
		t.Fatalf("normalizeGroupModelRows returned %d rows, want 1", len(got))
	}
	if got[0].Model != "gpt-image-2" {
		t.Fatalf("normalized model = %q, want gpt-image-2", got[0].Model)
	}
	if got[0].Enabled {
		t.Fatalf("normalized enabled = true, want false")
	}
}
