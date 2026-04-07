package model

import "testing"

func TestNormalizeChannelModelEndpoint(t *testing.T) {
	t.Run("text defaults to responses", func(t *testing.T) {
		if got := NormalizeChannelModelEndpoint(ProviderModelTypeText, ""); got != ChannelModelEndpointResponses {
			t.Fatalf("NormalizeChannelModelEndpoint(text, empty) = %q, want %q", got, ChannelModelEndpointResponses)
		}
		if got := NormalizeChannelModelEndpoint(ProviderModelTypeText, "/V1/CHAT/COMPLETIONS"); got != ChannelModelEndpointChat {
			t.Fatalf("NormalizeChannelModelEndpoint(text, chat) = %q, want %q", got, ChannelModelEndpointChat)
		}
		if got := NormalizeChannelModelEndpoint(ProviderModelTypeText, "/V1/MESSAGES"); got != ChannelModelEndpointMessages {
			t.Fatalf("NormalizeChannelModelEndpoint(text, messages) = %q, want %q", got, ChannelModelEndpointMessages)
		}
	})

	t.Run("image supports multiple endpoints", func(t *testing.T) {
		if got := NormalizeChannelModelEndpoint(ProviderModelTypeImage, ""); got != ChannelModelEndpointImages {
			t.Fatalf("NormalizeChannelModelEndpoint(image, empty) = %q, want %q", got, ChannelModelEndpointImages)
		}
		if got := NormalizeChannelModelEndpoint(ProviderModelTypeImage, ChannelModelEndpointResponses); got != ChannelModelEndpointResponses {
			t.Fatalf("NormalizeChannelModelEndpoint(image, responses) = %q, want %q", got, ChannelModelEndpointResponses)
		}
		if got := NormalizeChannelModelEndpoint(ProviderModelTypeImage, ChannelModelEndpointImageEdit); got != ChannelModelEndpointImageEdit {
			t.Fatalf("NormalizeChannelModelEndpoint(image, edits) = %q, want %q", got, ChannelModelEndpointImageEdit)
		}
		if got := NormalizeChannelModelEndpoint(ProviderModelTypeImage, ChannelModelEndpointBatches); got != ChannelModelEndpointBatches {
			t.Fatalf("NormalizeChannelModelEndpoint(image, batches) = %q, want %q", got, ChannelModelEndpointBatches)
		}
	})

	t.Run("other non-text endpoints are fixed by type", func(t *testing.T) {
		if got := NormalizeChannelModelEndpoint(ProviderModelTypeAudio, ""); got != ChannelModelEndpointAudio {
			t.Fatalf("NormalizeChannelModelEndpoint(audio, empty) = %q, want %q", got, ChannelModelEndpointAudio)
		}
		if got := NormalizeChannelModelEndpoint(ProviderModelTypeVideo, ""); got != ChannelModelEndpointVideos {
			t.Fatalf("NormalizeChannelModelEndpoint(video, empty) = %q, want %q", got, ChannelModelEndpointVideos)
		}
	})
}

func TestNormalizeChannelTestRowsKeepsLatestByModelAndEndpoint(t *testing.T) {
	rows := NormalizeChannelTestRows([]ChannelTest{
		{
			ChannelId: "channel-1",
			Model:     "gpt-4.1",
			Type:      ProviderModelTypeText,
			Endpoint:  ChannelModelEndpointResponses,
			Status:    ChannelTestStatusUnsupported,
			Supported: false,
			TestedAt:  10,
		},
		{
			ChannelId: " channel-1 ",
			Model:     " gpt-4.1 ",
			Type:      ProviderModelTypeText,
			Endpoint:  ChannelModelEndpointResponses,
			Status:    ChannelTestStatusSupported,
			Supported: true,
			TestedAt:  20,
		},
		{
			ChannelId: "channel-1",
			Model:     "gpt-4.1",
			Type:      ProviderModelTypeText,
			Endpoint:  ChannelModelEndpointChat,
			Status:    ChannelTestStatusSupported,
			Supported: true,
			TestedAt:  15,
		},
	})

	if len(rows) != 2 {
		t.Fatalf("NormalizeChannelTestRows returned %d rows, want 2", len(rows))
	}
	if rows[0].Endpoint != ChannelModelEndpointResponses || rows[0].Status != ChannelTestStatusSupported || !rows[0].Supported || rows[0].TestedAt != 20 {
		t.Fatalf("unexpected normalized row[0]: %#v", rows[0])
	}
	if rows[1].Endpoint != ChannelModelEndpointChat {
		t.Fatalf("unexpected normalized row[1] endpoint: %#v", rows[1])
	}
}

func TestNormalizeChannelTestRowsKeepsDistinctRounds(t *testing.T) {
	rows := NormalizeChannelTestRows([]ChannelTest{
		{
			ChannelId: "channel-1",
			Model:     "gpt-4.1",
			Round:     1,
			Type:      ProviderModelTypeText,
			Endpoint:  ChannelModelEndpointResponses,
			Status:    ChannelTestStatusSupported,
			Supported: true,
			TestedAt:  10,
		},
		{
			ChannelId: "channel-1",
			Model:     "gpt-4.1",
			Round:     2,
			Type:      ProviderModelTypeText,
			Endpoint:  ChannelModelEndpointChat,
			Status:    ChannelTestStatusUnsupported,
			Supported: false,
			TestedAt:  20,
		},
	})

	if len(rows) != 2 {
		t.Fatalf("NormalizeChannelTestRows returned %d rows, want 2", len(rows))
	}
	if rows[0].Round != 1 || rows[1].Round != 2 {
		t.Fatalf("unexpected rounds after normalization: %#v", rows)
	}
}
