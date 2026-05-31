package tokenestimate

import (
	"encoding/json"
	"slices"
	"testing"

	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func TestExtractStructuredMetaResponsesInput(t *testing.T) {
	req := EstimateRequest{
		RelayMode: relaymode.Responses,
		Model:     "gpt-4o",
		Request: &relaymodel.GeneralOpenAIRequest{
			Model: "gpt-4o",
			Input: []any{
				map[string]any{
					"role": "user",
					"type": "message",
					"name": "planner",
					"content": []any{
						map[string]any{
							"type": "input_text",
							"text": "summarize this document",
						},
						map[string]any{
							"type":       "function_call",
							"name":       "search_docs",
							"call_id":    "call_123",
							"arguments":  map[string]any{"query": "router token estimate"},
							"parameters": map[string]any{"type": "object", "required": []any{"query"}},
						},
					},
				},
				map[string]any{
					"role":         "assistant",
					"type":         "reasoning",
					"summary":      "need more context",
					"output_text":  "draft answer",
					"instructions": "be concise",
				},
			},
			Tools: []relaymodel.Tool{
				{
					Type: "function",
					Function: relaymodel.Function{
						Name:        "search_docs",
						Description: "search internal documents",
						Parameters:  map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}},
					},
				},
			},
		},
	}

	meta := extractStructuredMeta(req)
	wantTexts := []string{
		"user",
		"message",
		"planner",
		"summarize this document",
		"function_call",
		"search_docs",
		"call_123",
		`{"query":"router token estimate"}`,
		`{"required":["query"],"type":"object"}`,
		"assistant",
		"reasoning",
		"need more context",
		"draft answer",
		"be concise",
	}
	for _, want := range wantTexts {
		if !slices.Contains(meta.Texts, want) {
			t.Fatalf("meta.Texts missing %q: %#v", want, meta.Texts)
		}
	}
	if !slices.Contains(meta.ToolTexts, "search_docs") {
		t.Fatalf("meta.ToolTexts missing tool name: %#v", meta.ToolTexts)
	}
	if !slices.Contains(meta.ToolTexts, "search internal documents") {
		t.Fatalf("meta.ToolTexts missing tool description: %#v", meta.ToolTexts)
	}
	if meta.ToolsCount != 1 {
		t.Fatalf("ToolsCount = %d, want 1", meta.ToolsCount)
	}
}

func TestExtractStructuredMetaResponsesTopLevelInstructions(t *testing.T) {
	req := EstimateRequest{
		RelayMode: relaymode.Responses,
		Model:     "gpt-5.4",
		Request: &relaymodel.GeneralOpenAIRequest{
			Model:        "gpt-5.4",
			Instructions: "reply in exactly one sentence",
			Input:        "summarize the incident",
		},
	}

	meta := extractStructuredMeta(req)
	if !slices.Contains(meta.Texts, "reply in exactly one sentence") {
		t.Fatalf("meta.Texts missing top-level instructions: %#v", meta.Texts)
	}
	if !slices.Contains(meta.Texts, "summarize the incident") {
		t.Fatalf("meta.Texts missing input text: %#v", meta.Texts)
	}
}

func TestExtractAnthropicMetaRaw(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":"You are a router assistant.",
		"messages":[
			{
				"role":"user",
				"content":[
					{"type":"text","text":"Find the quota policy"},
					{"type":"tool_result","tool_use_id":"toolu_1","content":"policy from cache"}
				]
			},
			{
				"role":"assistant",
				"content":[
					{"type":"text","text":"I will inspect the policy."},
					{"type":"tool_use","id":"toolu_1","name":"search_docs","input":{"query":"quota policy"}}
				]
			}
		],
		"tools":[
			{
				"name":"search_docs",
				"description":"search docs",
				"input_schema":{
					"type":"object",
					"properties":{"query":{"type":"string"}},
					"required":["query"]
				}
			}
		]
	}`)

	meta, err := extractAnthropicMeta(raw)
	if err != nil {
		t.Fatalf("extractAnthropicMeta returned error: %v", err)
	}

	wantTexts := []string{
		"You are a router assistant.",
		"user",
		"text",
		"Find the quota policy",
		"tool_result",
		"toolu_1",
		"policy from cache",
		"assistant",
		"text",
		"I will inspect the policy.",
		"tool_use",
		"toolu_1",
		"search_docs",
		`{"query":"quota policy"}`,
	}
	for _, want := range wantTexts {
		if !slices.Contains(meta.Texts, want) {
			t.Fatalf("meta.Texts missing %q: %#v", want, meta.Texts)
		}
	}
	if meta.MessagesCount != 2 {
		t.Fatalf("MessagesCount = %d, want 2", meta.MessagesCount)
	}
	if meta.ToolsCount != 1 {
		t.Fatalf("ToolsCount = %d, want 1", meta.ToolsCount)
	}
}

func TestExtractGeminiMetaRaw(t *testing.T) {
	raw := []byte(`{
		"systemInstruction": {
			"parts": [{"text": "You are a concise assistant."}]
		},
		"contents": [
			{
				"role": "user",
				"parts": [
					{"text": "Summarize this incident"},
					{"fileData": {"mimeType": "text/plain", "fileUri": "gs://bucket/incident.txt"}}
				]
			},
			{
				"role": "model",
				"parts": [
					{"functionCall": {"name": "search_docs", "args": {"query": "incident"}}}
				]
			}
		],
		"tools": [
			{
				"functionDeclarations": [
					{
						"name": "search_docs",
						"description": "search docs",
						"parameters": {
							"type": "object",
							"properties": {"query": {"type": "string"}}
						}
					}
				]
			}
		],
		"generationConfig": {"responseMimeType": "application/json"}
	}`)

	meta, err := extractGeminiMeta(raw)
	if err != nil {
		t.Fatalf("extractGeminiMeta returned error: %v", err)
	}

	wantTexts := []string{
		"You are a concise assistant.",
		"user",
		"Summarize this incident",
		"text/plain",
		"gs://bucket/incident.txt",
		"model",
		"search_docs",
		`{"query":"incident"}`,
		"search docs",
		`{"properties":{"query":{"type":"string"}},"type":"object"}`,
		`{"responseMimeType":"application/json"}`,
	}
	for _, want := range wantTexts {
		if !slices.Contains(meta.Texts, want) {
			t.Fatalf("meta.Texts missing %q: %#v", want, meta.Texts)
		}
	}
	if meta.MessagesCount != 2 {
		t.Fatalf("MessagesCount = %d, want 2", meta.MessagesCount)
	}
	if meta.ToolsCount != 1 {
		t.Fatalf("ToolsCount = %d, want 1", meta.ToolsCount)
	}
}

func TestAppendAnyTextMapDeterministicJSON(t *testing.T) {
	meta := EstimateMeta{}
	value := map[string]any{
		"b": "second",
		"a": []any{"first", map[string]any{"z": "nested"}},
	}

	appendAnyText(&meta, value)

	if len(meta.Texts) != 1 {
		t.Fatalf("len(meta.Texts) = %d, want 1", len(meta.Texts))
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(meta.Texts[0]), &decoded); err != nil {
		t.Fatalf("appendAnyText produced invalid json %q: %v", meta.Texts[0], err)
	}
	if decoded["b"] != "second" {
		t.Fatalf("decoded[b] = %#v, want second", decoded["b"])
	}
}
