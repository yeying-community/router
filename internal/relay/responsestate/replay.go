package responsestate

import (
	"encoding/json"
	"strings"
)

// PrepareToolReplay accepts only a self-contained text/local-function window.
// It does not fetch history or make encrypted/provider-owned state portable.
// Only input item IDs are removed; call_id and application data stay unchanged.
func PrepareToolReplay(raw []byte) ([]byte, bool) {
	var root map[string]json.RawMessage
	if len(raw) > 16*1024*1024 || json.Unmarshal(raw, &root) != nil || root == nil {
		return nil, false
	}
	for _, key := range []string{"previous_response_id", "conversation", "encrypted_content"} {
		if v := root[key]; len(v) > 0 && string(v) != "null" && string(v) != `""` {
			return nil, false
		}
	}
	if v := root["background"]; len(v) > 0 && string(v) != "false" && string(v) != "null" {
		return nil, false
	}
	var tools []map[string]json.RawMessage
	if v := root["tools"]; len(v) > 0 {
		if json.Unmarshal(v, &tools) != nil {
			return nil, false
		}
		for _, tool := range tools {
			if replayString(tool["type"]) != "function" {
				return nil, false
			}
		}
	}
	var input []map[string]json.RawMessage
	if json.Unmarshal(root["input"], &input) != nil || len(input) == 0 {
		return nil, false
	}
	calls := make(map[string]bool)
	hasUser, hasOutput := false, false
	for _, item := range input {
		if item == nil || len(item["encrypted_content"]) != 0 {
			return nil, false
		}
		switch replayString(item["type"]) {
		case "", "message":
			role := replayString(item["role"])
			if role != "user" && role != "assistant" && role != "system" && role != "developer" {
				return nil, false
			}
			text, ok := replayMessageText(item["content"])
			if !ok {
				return nil, false
			}
			hasUser = hasUser || role == "user" && strings.TrimSpace(text) != ""
		case "function_call":
			id := replayString(item["call_id"])
			_, exists := calls[id]
			_, argumentsOK := replayText(item["arguments"])
			status := replayString(item["status"])
			if id == "" || exists || replayString(item["name"]) == "" ||
				!argumentsOK ||
				(status != "" && status != "completed") {
				return nil, false
			}
			calls[id] = false
		case "function_call_output":
			id := replayString(item["call_id"])
			completed, exists := calls[id]
			_, outputOK := replayText(item["output"])
			if !exists || completed || !outputOK {
				return nil, false
			}
			calls[id] = true
			hasOutput = true
		default:
			// References, compaction/reasoning, hosted tools and media need their
			// own portability rules. Retain the pin rather than guessing.
			return nil, false
		}
		delete(item, "id")
	}
	if !hasUser || !hasOutput {
		return nil, false
	}
	for _, completed := range calls {
		if !completed {
			return nil, false
		}
	}
	updated, err := json.Marshal(input)
	if err != nil {
		return nil, false
	}
	root["input"] = updated
	delete(root, "previous_response_id")
	result, err := json.Marshal(root)
	return result, err == nil
}

func replayString(raw json.RawMessage) string {
	value, _ := replayText(raw)
	return value
}

func replayText(raw json.RawMessage) (string, bool) {
	var value *string
	if json.Unmarshal(raw, &value) != nil || value == nil {
		return "", false
	}
	return *value, true
}

func replayMessageText(raw json.RawMessage) (string, bool) {
	if text, ok := replayText(raw); ok {
		return text, true
	}
	var parts []map[string]json.RawMessage
	if json.Unmarshal(raw, &parts) != nil || parts == nil {
		return "", false
	}
	var result strings.Builder
	for _, part := range parts {
		kind := replayString(part["type"])
		if kind != "input_text" && kind != "output_text" {
			return "", false
		}
		if v := part["annotations"]; len(v) > 0 && string(v) != "[]" && string(v) != "null" {
			return "", false
		}
		text, ok := replayText(part["text"])
		if !ok {
			return "", false
		}
		result.WriteString(text)
	}
	return result.String(), true
}
