package channel

import (
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
)

func TestMergeMissingProviderDetailsAsDeleted(t *testing.T) {
	current := []model.ProviderModelDetail{
		{
			Model:       "gpt-5.5",
			Type:        model.ProviderModelTypeText,
			Description: "active",
		},
	}
	existing := []model.ProviderModelDetail{
		{
			Model:       "gpt-5.5",
			Type:        model.ProviderModelTypeText,
			Description: "active",
		},
		{
			Model:       "gpt-4.1",
			Type:        model.ProviderModelTypeText,
			Description: "legacy",
		},
	}

	merged := mergeMissingProviderDetailsAsDeleted(current, existing)
	if len(merged) != 2 {
		t.Fatalf("mergeMissingProviderDetailsAsDeleted len=%d, want 2", len(merged))
	}

	byModel := make(map[string]model.ProviderModelDetail, len(merged))
	for _, detail := range merged {
		byModel[detail.Model] = detail
	}
	if byModel["gpt-5.5"].IsDeleted {
		t.Fatalf("gpt-5.5 should remain active")
	}
	if !byModel["gpt-4.1"].IsDeleted {
		t.Fatalf("gpt-4.1 should be soft deleted")
	}
	if byModel["gpt-4.1"].Description != "legacy" {
		t.Fatalf("soft deleted detail should preserve original fields, got description=%q", byModel["gpt-4.1"].Description)
	}
}
