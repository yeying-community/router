package anthropic

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
)

type inboundMessagesRequest struct {
	Model         string                     `json:"model"`
	Messages      []inboundMessagesItem      `json:"messages"`
	System        any                        `json:"system,omitempty"`
	MaxTokens     int                        `json:"max_tokens,omitempty"`
	StopSequences []string                   `json:"stop_sequences,omitempty"`
	Stream        bool                       `json:"stream,omitempty"`
	Temperature   *float64                   `json:"temperature,omitempty"`
	TopP          *float64                   `json:"top_p,omitempty"`
	TopK          int                        `json:"top_k,omitempty"`
	Tools         []Tool                     `json:"tools,omitempty"`
	ToolChoice    any                        `json:"tool_choice,omitempty"`
	Metadata      map[string]json.RawMessage `json:"metadata,omitempty"`
}

type inboundMessagesItem struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type MessagesRequestMeta struct {
	Model     string
	MaxTokens int
	Stream    bool
}

func ValidateMessagesRequest(raw []byte) error {
	request := &inboundMessagesRequest{}
	if err := json.Unmarshal(raw, request); err != nil {
		return err
	}
	return validateMessagesRequest(request)
}

func ParseMessagesRequestMeta(raw []byte) (*MessagesRequestMeta, error) {
	request := &inboundMessagesRequest{}
	if err := json.Unmarshal(raw, request); err != nil {
		return nil, err
	}
	if err := validateMessagesRequest(request); err != nil {
		return nil, err
	}
	return &MessagesRequestMeta{
		Model:     strings.TrimSpace(request.Model),
		MaxTokens: request.MaxTokens,
		Stream:    request.Stream,
	}, nil
}

func validateMessagesRequest(request *inboundMessagesRequest) error {
	if request == nil {
		return errors.New("request is nil")
	}
	if request.MaxTokens < 0 || request.MaxTokens > math.MaxInt32/2 {
		return errors.New("max_tokens is invalid")
	}
	if strings.TrimSpace(request.Model) == "" {
		return errors.New("field model is required")
	}
	if len(request.Messages) == 0 {
		return errors.New("field messages is required")
	}
	return nil
}
