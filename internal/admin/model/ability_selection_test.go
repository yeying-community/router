package model

import "testing"

func TestSelectRandomSatisfiedChannelExcludesFailedChannels(t *testing.T) {
	channels := []*Channel{
		{Id: "channel-a"},
		{Id: "channel-b"},
		{Id: "channel-c"},
	}

	got := SelectRandomSatisfiedChannel(channels, false, map[string]struct{}{
		"channel-a": {},
		"channel-b": {},
	})

	if got == nil {
		t.Fatalf("expected channel, got nil")
	}
	if got.Id != "channel-c" {
		t.Fatalf("unexpected channel id: got %q want %q", got.Id, "channel-c")
	}
}

func TestSelectRandomSatisfiedChannelIgnoresFirstPriorityLayerWhenRequested(t *testing.T) {
	high := int64(10)
	low := int64(1)
	channels := []*Channel{
		{Id: "channel-a", Priority: &high},
		{Id: "channel-b", Priority: &high},
		{Id: "channel-c", Priority: &low},
	}

	got := SelectRandomSatisfiedChannel(channels, true, nil)

	if got == nil {
		t.Fatalf("expected channel, got nil")
	}
	if got.Id != "channel-c" {
		t.Fatalf("unexpected channel id: got %q want %q", got.Id, "channel-c")
	}
}

func TestSelectRandomSatisfiedChannelKeepsCurrentPriorityTierBeforeDowngrading(t *testing.T) {
	high := int64(10)
	low := int64(1)
	channels := []*Channel{
		{Id: "channel-a", Priority: &high},
		{Id: "channel-b", Priority: &high},
		{Id: "channel-c", Priority: &low},
	}

	got, stats := SelectRandomSatisfiedChannelWithStats(channels, false, map[string]struct{}{
		"channel-a": {},
	})

	if got == nil {
		t.Fatalf("expected channel, got nil")
	}
	if got.Id != "channel-b" {
		t.Fatalf("unexpected channel id: got %q want %q", got.Id, "channel-b")
	}
	if stats.SelectionScope != "same_priority" {
		t.Fatalf("unexpected selection scope: got %q", stats.SelectionScope)
	}
	if stats.SelectedTierCandidates != 1 {
		t.Fatalf("unexpected selected tier candidates: got %d want %d", stats.SelectedTierCandidates, 1)
	}
	if stats.RemainingCandidates != 2 {
		t.Fatalf("unexpected remaining candidates: got %d want %d", stats.RemainingCandidates, 2)
	}
}

func TestSelectRandomSatisfiedChannelDowngradesWhenCurrentPriorityTierExhausted(t *testing.T) {
	high := int64(10)
	low := int64(1)
	channels := []*Channel{
		{Id: "channel-a", Priority: &high},
		{Id: "channel-b", Priority: &high},
		{Id: "channel-c", Priority: &low},
	}

	got, stats := SelectRandomSatisfiedChannelWithStats(channels, false, map[string]struct{}{
		"channel-a": {},
		"channel-b": {},
	})

	if got == nil {
		t.Fatalf("expected channel, got nil")
	}
	if got.Id != "channel-c" {
		t.Fatalf("unexpected channel id: got %q want %q", got.Id, "channel-c")
	}
	if stats.SelectionScope != "downgraded" {
		t.Fatalf("unexpected selection scope: got %q", stats.SelectionScope)
	}
	if stats.SelectedTierCandidates != 1 {
		t.Fatalf("unexpected selected tier candidates: got %d want %d", stats.SelectedTierCandidates, 1)
	}
	if stats.RemainingCandidates != 1 {
		t.Fatalf("unexpected remaining candidates: got %d want %d", stats.RemainingCandidates, 1)
	}
}

func TestSelectRandomSatisfiedChannelReturnsNilWhenAllTiersExhausted(t *testing.T) {
	channels := []*Channel{
		{Id: "channel-a"},
		{Id: "channel-b"},
	}

	got, stats := SelectRandomSatisfiedChannelWithStats(channels, false, map[string]struct{}{
		"channel-a": {},
		"channel-b": {},
	})

	if got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
	if stats.SelectionScope != "candidate_exhausted" {
		t.Fatalf("unexpected selection scope: got %q", stats.SelectionScope)
	}
	if stats.RemainingCandidates != 0 {
		t.Fatalf("unexpected remaining candidates: got %d want %d", stats.RemainingCandidates, 0)
	}
}

func TestSelectRandomSatisfiedChannelUsesRoutePriorityOverride(t *testing.T) {
	basePriority := int64(0)
	channels := []*Channel{
		CloneChannelWithPriority(&Channel{Id: "channel-a", Priority: &basePriority}, 1),
		CloneChannelWithPriority(&Channel{Id: "channel-b", Priority: &basePriority}, 1),
		CloneChannelWithPriority(&Channel{Id: "channel-c", Priority: &basePriority}, 0),
	}

	got, stats := SelectRandomSatisfiedChannelWithStats(channels, false, nil)
	if got == nil {
		t.Fatalf("expected channel, got nil")
	}
	if got.Id != "channel-a" && got.Id != "channel-b" {
		t.Fatalf("unexpected channel id: got %q want tier-1 candidate", got.Id)
	}
	if stats.SelectedPriority != 1 {
		t.Fatalf("selected priority = %d, want 1", stats.SelectedPriority)
	}
	if stats.SelectedTierCandidates != 2 {
		t.Fatalf("selected tier candidates = %d, want 2", stats.SelectedTierCandidates)
	}
}

func TestResolveRuntimeChannelPriorityDemotesHalfOpen(t *testing.T) {
	priority := resolveRuntimeChannelPriority(&Channel{Status: ChannelStatusHalfOpen}, 100)
	if priority != ChannelHalfOpenPriority {
		t.Fatalf("half-open priority = %d, want %d", priority, ChannelHalfOpenPriority)
	}
	enabledPriority := resolveRuntimeChannelPriority(&Channel{Status: ChannelStatusEnabled}, 100)
	if enabledPriority != 100 {
		t.Fatalf("enabled priority = %d, want 100", enabledPriority)
	}
}
