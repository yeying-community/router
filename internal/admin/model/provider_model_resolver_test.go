package model

import "testing"

func TestNormalizeProviderLookupCandidates(t *testing.T) {
	values := NormalizeProviderLookupCandidates(
		"",
		"gpt-5.4",
		"openai/gpt-5.4",
		"openai/gpt-5.4",
		"claude-opus-4-6",
	)
	want := []string{"gpt-5.4", "openai/gpt-5.4", "claude-opus-4-6"}
	if len(values) != len(want) {
		t.Fatalf("len(candidates)=%d, want=%d candidates=%v", len(values), len(want), values)
	}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("candidates[%d]=%q, want=%q all=%v", i, values[i], want[i], values)
		}
	}
}

func TestResolveProviderFromCatalogMap(t *testing.T) {
	catalog := map[string]string{
		"gpt-5.4":      "openai",
		"claude-4.1":   "anthropic",
		"legacy-model": "custom",
	}
	if got := ResolveProviderFromCatalogMap(catalog, "openai/gpt-5.4"); got != "openai" {
		t.Fatalf("ResolveProviderFromCatalogMap(openai/gpt-5.4)=%q, want openai", got)
	}
	if got := ResolveProviderFromCatalogMap(catalog, "claude-4.1"); got != "anthropic" {
		t.Fatalf("ResolveProviderFromCatalogMap(claude-4.1)=%q, want anthropic", got)
	}
	if got := ResolveProviderFromCatalogMap(catalog, "unknown-model"); got != "" {
		t.Fatalf("ResolveProviderFromCatalogMap(unknown-model)=%q, want empty", got)
	}
	if got := ResolveProviderFromCatalogMap(catalog, "legacy-model"); got != "" {
		t.Fatalf("ResolveProviderFromCatalogMap(legacy-model)=%q, want empty", got)
	}
}
