package responsestate

import (
	"testing"
	"time"
)

func TestAnalyzeRequestBodyDetectsPreviousResponseAndToolOutput(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-5.4",
		"previous_response_id":"resp_prev",
		"input":[
			{"type":"message","role":"user","content":"hello"},
			{"type":"function_call_output","call_id":"call_123","output":"ok"}
		]
	}`)

	state := AnalyzeRequestBody(raw)
	if state.PreviousResponseID != "resp_prev" {
		t.Fatalf("PreviousResponseID = %q, want resp_prev", state.PreviousResponseID)
	}
	if !state.HasToolOutput {
		t.Fatal("HasToolOutput = false, want true")
	}
	if !IsStatefulRequestBody(raw) {
		t.Fatal("IsStatefulRequestBody = false, want true")
	}
}

func TestStoreAndLookupRoute(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	StoreRoute(" resp_1 ", " channel-1 ")
	channelID, ok := LookupRoute("resp_1")
	if !ok {
		t.Fatal("LookupRoute ok = false, want true")
	}
	if channelID != "channel-1" {
		t.Fatalf("LookupRoute channel = %q, want channel-1", channelID)
	}
}

func TestLookupRouteExpires(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	now := time.Unix(100, 0)
	routeNow = func() time.Time { return now }
	routeTTL = time.Second

	StoreRoute("resp_1", "channel-1")
	now = now.Add(2 * time.Second)
	if channelID, ok := LookupRoute("resp_1"); ok {
		t.Fatalf("LookupRoute = (%q, true), want expired", channelID)
	}
}

func TestExtractResponseID(t *testing.T) {
	if got := ExtractResponseID([]byte(`{"id":"resp_abc","object":"response"}`)); got != "resp_abc" {
		t.Fatalf("ExtractResponseID = %q, want resp_abc", got)
	}
}
