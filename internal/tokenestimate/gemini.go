package tokenestimate

import (
	"encoding/json"
	"fmt"
	"strings"
)

type geminiEstimateRequest struct {
	SystemInstruction geminiContent    `json:"systemInstruction,omitempty"`
	Contents          []geminiContent  `json:"contents,omitempty"`
	Tools             []geminiTool     `json:"tools,omitempty"`
	GenerationConfig  map[string]any   `json:"generationConfig,omitempty"`
	SafetySettings    []map[string]any `json:"safetySettings,omitempty"`
	CachedContent     string           `json:"cachedContent,omitempty"`
	ToolConfig        map[string]any   `json:"toolConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts,omitempty"`
}

type geminiPart struct {
	Text             string         `json:"text,omitempty"`
	FunctionCall     geminiFunction `json:"functionCall,omitempty"`
	FunctionResponse geminiFunction `json:"functionResponse,omitempty"`
	InlineData       geminiBlob     `json:"inlineData,omitempty"`
	FileData         geminiBlob     `json:"fileData,omitempty"`
	VideoMetadata    map[string]any `json:"videoMetadata,omitempty"`
}

type geminiFunction struct {
	Name     string `json:"name,omitempty"`
	Args     any    `json:"args,omitempty"`
	Response any    `json:"response,omitempty"`
}

type geminiBlob struct {
	MimeType string `json:"mimeType,omitempty"`
	FileURI  string `json:"fileUri,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type geminiFunctionDeclaration struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

func estimateGeminiFromRequest(req EstimateRequest, model string) (EstimateResult, error) {
	if req.Request != nil {
		return estimateHeuristicFromRequest(req, familyGemini, "local_gemini_heuristic", "gemini_heuristic")
	}
	meta, err := extractGeminiMeta(req.RawBody)
	if err != nil {
		return EstimateResult{}, err
	}
	return EstimateResult{
		PromptTokens: estimateTextsHeuristic(meta.Texts, familyGemini),
		Source:       "local_gemini_heuristic",
		Precision:    PrecisionHeuristic,
		Estimator:    "gemini_heuristic",
	}, nil
}

func extractGeminiMeta(raw []byte) (EstimateMeta, error) {
	meta := EstimateMeta{}
	if len(raw) == 0 {
		return meta, fmt.Errorf("gemini estimate request is empty")
	}
	req := geminiEstimateRequest{}
	if err := json.Unmarshal(raw, &req); err != nil {
		return meta, fmt.Errorf("unmarshal gemini request: %w", err)
	}
	extractGeminiContent(&meta, req.SystemInstruction)
	for _, content := range req.Contents {
		meta.MessagesCount++
		extractGeminiContent(&meta, content)
	}
	for _, tool := range req.Tools {
		for _, declaration := range tool.FunctionDeclarations {
			meta.ToolsCount++
			appendToolText(&meta, declaration.Name)
			appendToolText(&meta, declaration.Description)
			appendToolAnyText(&meta, declaration.Parameters)
		}
	}
	appendExtraAnyText(&meta, req.GenerationConfig)
	appendExtraAnyText(&meta, req.SafetySettings)
	appendExtraAnyText(&meta, req.ToolConfig)
	appendExtraText(&meta, req.CachedContent)
	return meta, nil
}

func extractGeminiContent(meta *EstimateMeta, content geminiContent) {
	appendText(meta, content.Role)
	for _, part := range content.Parts {
		extractGeminiPart(meta, part)
	}
}

func extractGeminiPart(meta *EstimateMeta, part geminiPart) {
	appendText(meta, part.Text)
	extractGeminiFunction(meta, part.FunctionCall)
	extractGeminiFunction(meta, part.FunctionResponse)
	appendText(meta, part.InlineData.MimeType)
	appendText(meta, part.FileData.MimeType)
	appendText(meta, part.FileData.FileURI)
	appendExtraAnyText(meta, part.VideoMetadata)
}

func extractGeminiFunction(meta *EstimateMeta, fn geminiFunction) {
	if strings.TrimSpace(fn.Name) == "" && fn.Args == nil && fn.Response == nil {
		return
	}
	appendText(meta, fn.Name)
	appendAnyText(meta, fn.Args)
	appendAnyText(meta, fn.Response)
}
