package channel

import (
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
)

func TestSanitizeChannelForResponseSetsKeyPreviewAndClearsKey(t *testing.T) {
	row := &model.Channel{
		Id:  "channel-1",
		Key: "sk-test-secret-value",
	}

	sanitizeChannelForResponse(row)

	if !row.KeySet {
		t.Fatalf("KeySet=false, want true")
	}
	if row.Key != "" {
		t.Fatalf("Key=%q, want empty", row.Key)
	}
	if row.KeyPreview != "sk-**************lue" {
		t.Fatalf("KeyPreview=%q, want sk-**************lue", row.KeyPreview)
	}
}

func TestSanitizeChannelForResponseMasksShortKey(t *testing.T) {
	row := &model.Channel{
		Id:  "channel-1",
		Key: "abc123",
	}

	sanitizeChannelForResponse(row)

	if row.KeyPreview != "******" {
		t.Fatalf("KeyPreview=%q, want ******", row.KeyPreview)
	}
}
