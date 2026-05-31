package tokenestimate

import (
	"fmt"
	"strings"
)

func estimateByProvider(req EstimateRequest, model string) (EstimateResult, error) {
	family := detectFamily(model)
	switch family {
	case familyOpenAI:
		return estimateOpenAIFromRequest(req, model)
	case familyAnthropic:
		if len(req.RawBody) == 0 {
			return EstimateResult{}, fmt.Errorf("anthropic estimate raw request body is empty")
		}
		return estimateAnthropicFromRaw(req, model)
	case familyGemini:
		return estimateGeminiFromRequest(req, model)
	default:
		return estimateHeuristicFromRequest(req, familyUnknown, "local_unknown_heuristic", "unknown_heuristic")
	}
}

func resolveEstimateModel(req EstimateRequest) string {
	model := strings.TrimSpace(req.Model)
	if model == "" && req.Request != nil {
		model = strings.TrimSpace(req.Request.Model)
	}
	return model
}
