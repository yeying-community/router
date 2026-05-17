package openai

import "strings"

type responsesOutputItem struct {
	Type string `json:"type,omitempty"`
}

type responsesBridgeEnvelope struct {
	Output []responsesOutputItem `json:"output,omitempty"`
}

func countResponsesImageGenerationCalls(envelope responsesBridgeEnvelope) int {
	count := 0
	for _, item := range envelope.Output {
		if strings.EqualFold(strings.TrimSpace(item.Type), "image_generation_call") {
			count++
		}
	}
	return count
}
