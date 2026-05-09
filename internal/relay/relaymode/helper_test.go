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

func TestGetByPath_ImagesGenerations(t *testing.T) {
	if got := GetByPath("/v1/images/generations"); got != ImagesGenerations {
		t.Fatalf("GetByPath(/v1/images/generations)=%d, want %d", got, ImagesGenerations)
	}
	if got := GetByPath("/api/v1/public/images/generations"); got != ImagesGenerations {
		t.Fatalf("GetByPath(/api/v1/public/images/generations)=%d, want %d", got, ImagesGenerations)
	}
}

func TestGetByPath_ImagesEdits(t *testing.T) {
	if got := GetByPath("/v1/images/edits"); got != ImagesEdits {
		t.Fatalf("GetByPath(/v1/images/edits)=%d, want %d", got, ImagesEdits)
	}
	if got := GetByPath("/api/v1/public/images/edits"); got != ImagesEdits {
		t.Fatalf("GetByPath(/api/v1/public/images/edits)=%d, want %d", got, ImagesEdits)
	}
}

func TestGetByPath_Audio(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{path: "/v1/audio/speech", want: AudioSpeech},
		{path: "/v1/audio/transcriptions", want: AudioTranscription},
		{path: "/v1/audio/translations", want: AudioTranslation},
		{path: "/api/v1/public/audio/speech", want: AudioSpeech},
		{path: "/api/v1/public/audio/transcriptions", want: AudioTranscription},
		{path: "/api/v1/public/audio/translations", want: AudioTranslation},
	}

	for _, tt := range tests {
		if got := GetByPath(tt.path); got != tt.want {
			t.Fatalf("GetByPath(%s)=%d, want %d", tt.path, got, tt.want)
		}
	}
}
