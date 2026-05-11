package model

import "testing"

func TestNormalizeGroupModelRouteRowsPreserveOrder_DeduplicatesByPrimaryKey(t *testing.T) {
	rows := []GroupModelRoute{
		{
			Group:         " group-a ",
			Model:         "gpt-4.1",
			ChannelId:     " channel-1 ",
			UpstreamModel: "upstream-a",
			Enabled:       true,
		},
		{
			Group:         "group-a",
			Model:         "gpt-4.1",
			ChannelId:     "channel-1",
			UpstreamModel: "upstream-b",
			Enabled:       false,
		},
		{
			Group:     "group-a",
			Model:     "gpt-4.1",
			ChannelId: "channel-2",
			Enabled:   true,
		},
	}

	got := normalizeGroupModelRouteRowsPreserveOrder(rows)
	if len(got) != 2 {
		t.Fatalf("normalizeGroupModelRouteRowsPreserveOrder returned %d rows, want 2", len(got))
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
