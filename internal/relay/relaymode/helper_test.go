package relaymode

import "testing"

func TestGetByPath_Videos(t *testing.T) {
	if got := GetByPath("/v1/videos"); got != Videos {
		t.Fatalf("GetByPath(/v1/videos)=%d, want %d", got, Videos)
	}
	if got := GetByPath("/v1/videos/task_123"); got != Videos {
		t.Fatalf("GetByPath(/v1/videos/task_123)=%d, want %d", got, Videos)
	}
	if got := GetByPath("/api/v1/public/videos"); got != Videos {
		t.Fatalf("GetByPath(/api/v1/public/videos)=%d, want %d", got, Videos)
	}
}

func TestGetByPath_Messages(t *testing.T) {
	if got := GetByPath("/v1/messages"); got != Messages {
		t.Fatalf("GetByPath(/v1/messages)=%d, want %d", got, Messages)
	}
	if got := GetByPath("/api/v1/public/messages"); got != Messages {
		t.Fatalf("GetByPath(/api/v1/public/messages)=%d, want %d", got, Messages)
	}
}
