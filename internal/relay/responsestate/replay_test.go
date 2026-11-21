package responsestate

import (
	"bytes"
	"encoding/json"
	"testing"
)

const replayFixture = `{"model":"test","metadata":{"id":"keep","big":9007199254740993},"tools":[{"type":"function","name":"read","parameters":{"type":"object","properties":{"id":{"type":"string"}}}}],"input":[{"type":"message","id":"msg_user","role":"user","content":[{"type":"input_text","text":"Read the file"}]},{"type":"function_call","id":"fc_old","call_id":"call_1","name":"read","arguments":"{\"id\":9007199254740993}","status":"completed"},{"type":"function_call_output","id":"fco_old","call_id":"call_1","output":"VALUE"}]}`

func TestPrepareToolReplayPreservesContentAndOnlyDetachesItemIDs(t *testing.T) {
	raw := []byte(replayFixture)
	before := append([]byte(nil), raw...)
	replay, ok := PrepareToolReplay(raw)
	if !ok || !bytes.Equal(raw, before) {
		t.Fatal("valid fixture rejected or source bytes changed")
	}
	var original, result map[string]json.RawMessage
	_ = json.Unmarshal(raw, &original)
	_ = json.Unmarshal(replay, &result)
	for _, key := range []string{"tools", "metadata", "model"} {
		if !bytes.Equal(original[key], result[key]) {
			t.Fatalf("application field %s changed", key)
		}
	}
	var input []map[string]json.RawMessage
	_ = json.Unmarshal(result["input"], &input)
	for _, item := range input {
		if _, exists := item["id"]; exists {
			t.Fatal("provider item ID retained")
		}
	}
	if replayString(input[1]["call_id"]) != "call_1" || replayString(input[2]["call_id"]) != "call_1" ||
		replayString(input[1]["arguments"]) != `{"id":9007199254740993}` || replayString(input[2]["output"]) != "VALUE" {
		t.Fatal("tool pairing or data changed")
	}
	if _, ok := PrepareToolReplay(replay); !ok {
		t.Fatal("detaching twice should preserve the valid window")
	}
}

func TestPrepareToolReplayRefusesNonPortableState(t *testing.T) {
	mutations := map[string]func(map[string]any){
		"parent response":  func(r map[string]any) { r["previous_response_id"] = "resp_A" },
		"conversation":     func(r map[string]any) { r["conversation"] = "conv_A" },
		"background":       func(r map[string]any) { r["background"] = true },
		"hosted tools":     func(r map[string]any) { r["tools"] = []any{map[string]any{"type": "code_interpreter"}} },
		"orphan output":    func(r map[string]any) { r["input"] = r["input"].([]any)[2:] },
		"out of order":     func(r map[string]any) { a := r["input"].([]any); a[1], a[2] = a[2], a[1] },
		"duplicate call":   func(r map[string]any) { a := r["input"].([]any); r["input"] = append(a, a[1]) },
		"duplicate output": func(r map[string]any) { a := r["input"].([]any); r["input"] = append(a, a[2]) },
		"pending call":     func(r map[string]any) { r["input"] = r["input"].([]any)[:2] },
		"no user":          func(r map[string]any) { r["input"] = r["input"].([]any)[1:] },
		"item reference": func(r map[string]any) {
			r["input"].([]any)[0] = map[string]any{"type": "item_reference", "id": "msg_A"}
		},
		"opaque compaction": func(r map[string]any) {
			r["input"] = append(r["input"].([]any), map[string]any{"type": "compaction", "encrypted_content": "OPAQUE"})
		},
		"reasoning": func(r map[string]any) {
			r["input"] = append(r["input"].([]any), map[string]any{"type": "reasoning", "summary": []any{}})
		},
		"image": func(r map[string]any) {
			r["input"].([]any)[0].(map[string]any)["content"] = []any{map[string]any{"type": "input_image", "file_id": "file_A"}}
		},
		"null tool result":  func(r map[string]any) { r["input"].([]any)[2].(map[string]any)["output"] = nil },
		"array tool result": func(r map[string]any) { r["input"].([]any)[2].(map[string]any)["output"] = []any{} },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var root map[string]any
			_ = json.Unmarshal([]byte(replayFixture), &root)
			mutate(root)
			raw, _ := json.Marshal(root)
			if result, ok := PrepareToolReplay(raw); ok || result != nil {
				t.Fatal("nonportable request accepted")
			}
		})
	}
}
