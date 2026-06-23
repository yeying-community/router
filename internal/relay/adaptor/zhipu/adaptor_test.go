package zhipu

import (
	"testing"

	"github.com/yeying-community/router/internal/relay/meta"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func TestGetRequestURLRealtimeUsesOfficialRealtimePath(t *testing.T) {
	adaptor := &Adaptor{}
	got, err := adaptor.GetRequestURL(&meta.Meta{
		Mode:    relaymode.Realtime,
		BaseURL: "https://open.bigmodel.cn/",
	})
	if err != nil {
		t.Fatalf("GetRequestURL() error = %v", err)
	}
	want := "https://open.bigmodel.cn/api/paas/v4/realtime"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}
