package anthropic

import (
	"strings"
	"testing"
)

func TestParseMessagesRequestToGeneralOpenAIRequestBasic(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":"you are helpful",
		"messages":[
			{"role":"user","content":"hello"}
		],
		"max_tokens":256,
		"stream":true,
		"temperature":0.2,
		"top_p":0.9,
		"top_k":40,
		"stop_sequences":["\n\nHuman:"],
		"tools":[
			{
				"name":"get_weather",
				"description":"Get weather by city",
				"input_schema":{
					"type":"object",
					"properties":{"city":{"type":"string"}},
					"required":["city"]
				}
			}
		],
		"tool_choice":{"type":"tool","name":"get_weather"}
	}`)

	request, err := ParseMessagesRequestToGeneralOpenAIRequest(raw)
	if err != nil {
		t.Fatalf("ParseMessagesRequestToGeneralOpenAIRequest returned error: %v", err)
	}
	if request.Model != "claude-sonnet-4-6" {
		t.Fatalf("request.Model = %q, want %q", request.Model, "claude-sonnet-4-6")
	}
	if request.MaxTokens != 256 {
		t.Fatalf("request.MaxTokens = %d, want %d", request.MaxTokens, 256)
	}
	if !request.Stream {
		t.Fatalf("request.Stream = false, want true")
	}
	if request.Temperature == nil || *request.Temperature != 0.2 {
		t.Fatalf("request.Temperature = %#v, want 0.2", request.Temperature)
	}
	if request.TopP == nil || *request.TopP != 0.9 {
		t.Fatalf("request.TopP = %#v, want 0.9", request.TopP)
	}
	if request.TopK != 40 {
		t.Fatalf("request.TopK = %d, want %d", request.TopK, 40)
	}
	if len(request.Messages) != 2 {
		t.Fatalf("len(request.Messages) = %d, want %d", len(request.Messages), 2)
	}
	if request.Messages[0].Role != "system" || request.Messages[0].StringContent() != "you are helpful" {
		t.Fatalf("request.Messages[0] = %#v, want system message", request.Messages[0])
	}
	if request.Messages[1].Role != "user" || request.Messages[1].StringContent() != "hello" {
		t.Fatalf("request.Messages[1] = %#v, want user message", request.Messages[1])
	}
	if len(request.Tools) != 1 || request.Tools[0].Function.Name != "get_weather" {
		t.Fatalf("request.Tools = %#v, want one get_weather tool", request.Tools)
	}
	params, ok := request.Tools[0].Function.Parameters.(map[string]any)
	if !ok {
		t.Fatalf("request.Tools[0].Function.Parameters = %#v, want map", request.Tools[0].Function.Parameters)
	}
	if params["type"] != "object" {
		t.Fatalf("params[type] = %#v, want %q", params["type"], "object")
	}
	required, ok := params["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "city" {
		t.Fatalf("params[required] = %#v, want [city]", params["required"])
	}
	stop, ok := request.Stop.([]string)
	if !ok || len(stop) != 1 || stop[0] != "\n\nHuman:" {
		t.Fatalf("request.Stop = %#v, want stop_sequences", request.Stop)
	}
	toolChoice, ok := request.ToolChoice.(map[string]any)
	if !ok {
		t.Fatalf("request.ToolChoice = %#v, want map", request.ToolChoice)
	}
	if toolChoice["type"] != "function" {
		t.Fatalf("toolChoice[type] = %#v, want %q", toolChoice["type"], "function")
	}
	function, ok := toolChoice["function"].(map[string]any)
	if !ok || function["name"] != "get_weather" {
		t.Fatalf("toolChoice[function] = %#v, want get_weather", toolChoice["function"])
	}
}

func TestParseMessagesRequestToGeneralOpenAIRequestRequiredDefaultsToEmptyArray(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"hello"}],
		"tools":[
			{
				"name":"no_required",
				"description":"tool without required",
				"input_schema":{
					"type":"object",
					"properties":{"q":{"type":"string"}}
				}
			},
			{
				"name":"null_required",
				"description":"tool with null required",
				"input_schema":{
					"type":"object",
					"properties":{"q":{"type":"string"}},
					"required":null
				}
			}
		]
	}`)

	request, err := ParseMessagesRequestToGeneralOpenAIRequest(raw)
	if err != nil {
		t.Fatalf("ParseMessagesRequestToGeneralOpenAIRequest returned error: %v", err)
	}
	if len(request.Tools) != 2 {
		t.Fatalf("len(request.Tools) = %d, want 2", len(request.Tools))
	}
	for i, tool := range request.Tools {
		params, ok := tool.Function.Parameters.(map[string]any)
		if !ok {
			t.Fatalf("request.Tools[%d].Function.Parameters = %#v, want map", i, tool.Function.Parameters)
		}
		required, ok := params["required"].([]string)
		if !ok {
			t.Fatalf("request.Tools[%d].Function.Parameters.required = %#v, want []string{}", i, params["required"])
		}
		if len(required) != 0 {
			t.Fatalf("request.Tools[%d].Function.Parameters.required = %#v, want empty array", i, required)
		}
	}
}

func TestParseMessagesRequestToGeneralOpenAIRequestToolUseAndResult(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{
				"role":"assistant",
				"content":[
					{"type":"text","text":"calling tool"},
					{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"q":"abc"}}
				]
			},
			{
				"role":"user",
				"content":[
					{"type":"tool_result","tool_use_id":"toolu_1","content":"{\"ok\":true}"}
				]
			}
		]
	}`)

	request, err := ParseMessagesRequestToGeneralOpenAIRequest(raw)
	if err != nil {
		t.Fatalf("ParseMessagesRequestToGeneralOpenAIRequest returned error: %v", err)
	}
	if len(request.Messages) != 2 {
		t.Fatalf("len(request.Messages) = %d, want %d", len(request.Messages), 2)
	}
	first := request.Messages[0]
	if first.Role != "assistant" {
		t.Fatalf("first.Role = %q, want %q", first.Role, "assistant")
	}
	if first.StringContent() != "calling tool" {
		t.Fatalf("first.StringContent() = %q, want %q", first.StringContent(), "calling tool")
	}
	if len(first.ToolCalls) != 1 {
		t.Fatalf("len(first.ToolCalls) = %d, want %d", len(first.ToolCalls), 1)
	}
	if first.ToolCalls[0].Id != "toolu_1" || first.ToolCalls[0].Function.Name != "lookup" {
		t.Fatalf("first.ToolCalls[0] = %#v, want lookup/toolu_1", first.ToolCalls[0])
	}
	second := request.Messages[1]
	if second.Role != "tool" {
		t.Fatalf("second.Role = %q, want %q", second.Role, "tool")
	}
	if second.ToolCallId != "toolu_1" {
		t.Fatalf("second.ToolCallId = %q, want %q", second.ToolCallId, "toolu_1")
	}
	if second.StringContent() != "{\"ok\":true}" {
		t.Fatalf("second.StringContent() = %q, want %q", second.StringContent(), "{\"ok\":true}")
	}
}

func TestParseMessagesRequestToGeneralOpenAIRequestImageContent(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{
				"role":"user",
				"content":[
					{"type":"text","text":"analyze this image"},
					{
						"type":"image",
						"source":{
							"type":"base64",
							"media_type":"image/png",
							"data":"AAA="
						}
					}
				]
			}
		]
	}`)

	request, err := ParseMessagesRequestToGeneralOpenAIRequest(raw)
	if err != nil {
		t.Fatalf("ParseMessagesRequestToGeneralOpenAIRequest returned error: %v", err)
	}
	if len(request.Messages) != 1 {
		t.Fatalf("len(request.Messages) = %d, want %d", len(request.Messages), 1)
	}
	contentParts, ok := request.Messages[0].Content.([]any)
	if !ok || len(contentParts) != 2 {
		t.Fatalf("request.Messages[0].Content = %#v, want two content parts", request.Messages[0].Content)
	}
	imagePart, ok := contentParts[1].(map[string]any)
	if !ok {
		t.Fatalf("contentParts[1] = %#v, want map", contentParts[1])
	}
	imageURL, ok := imagePart["image_url"].(map[string]any)
	if !ok {
		t.Fatalf("imagePart[image_url] = %#v, want map", imagePart["image_url"])
	}
	url, ok := imageURL["url"].(string)
	if !ok || !strings.HasPrefix(url, "data:image/png;base64,AAA=") {
		t.Fatalf("image_url.url = %#v, want data URI", imageURL["url"])
	}
}
