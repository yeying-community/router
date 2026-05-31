package openai

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"unicode"

	"github.com/pkoukk/tiktoken-go"

	"github.com/yeying-community/router/common/image"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/relay/model"
)

const (
	defaultTokenizerEncodingName = "cl100k_base"
	modernTokenizerEncodingName  = "o200k_base"
)

var (
	tokenEncoderMu      sync.RWMutex
	tokenEncoderMap     = map[string]*tiktoken.Tiktoken{}
	defaultTokenEncoder *tiktoken.Tiktoken
)

type tokenizerEncodingRule struct {
	Encoding string
	Prefixes []string
	Exact    []string
	Match    func(string) bool
}

var tokenizerEncodingRules = []tokenizerEncodingRule{
	{
		Encoding: modernTokenizerEncodingName,
		Prefixes: []string{
			"gpt-4o",
			"chatgpt-4o",
			"gpt-4.1",
			"gpt-4.5",
			"gpt-5",
			"gpt-realtime",
			"gpt-audio",
			"gpt-image",
			"chatgpt-image",
		},
		Match: isOReasoningModelName,
	},
	{
		Encoding: defaultTokenizerEncodingName,
		Prefixes: []string{
			"gpt-4",
			"gpt-3.5",
			"text-embedding-3-",
		},
		Exact: []string{
			"text-embedding-ada-002",
		},
	},
	{
		Encoding: "p50k_base",
		Prefixes: []string{
			"text-davinci-00",
			"code-davinci-",
			"code-cushman-",
		},
		Exact: []string{
			"davinci-codex",
			"cushman-codex",
		},
	},
	{
		Encoding: "p50k_edit",
		Exact: []string{
			"text-davinci-edit-001",
			"code-davinci-edit-001",
		},
	},
	{
		Encoding: "r50k_base",
		Prefixes: []string{
			"text-davinci-001",
			"text-curie-001",
			"text-babbage-001",
			"text-ada-001",
			"text-similarity-",
			"text-search-",
			"code-search-",
		},
		Exact: []string{
			"davinci",
			"curie",
			"babbage",
			"ada",
		},
	},
}

func resolveTokenizerEncodingName(model string) string {
	normalized := strings.TrimSpace(strings.ToLower(model))
	if normalized == "" {
		return ""
	}
	for _, rule := range tokenizerEncodingRules {
		for _, exact := range rule.Exact {
			if normalized == exact {
				return rule.Encoding
			}
		}
		for _, prefix := range rule.Prefixes {
			if strings.HasPrefix(normalized, prefix) {
				return rule.Encoding
			}
		}
		if rule.Match != nil && rule.Match(normalized) {
			return rule.Encoding
		}
	}
	return resolveTokenizerFamilyFallbackEncodingName(normalized)
}

func resolveTokenizerFamilyFallbackEncodingName(model string) string {
	normalized := strings.TrimSpace(strings.ToLower(model))
	switch {
	case normalized == "":
		return ""
	case strings.HasPrefix(normalized, "gpt-"):
		return modernTokenizerEncodingName
	case isOReasoningModelName(normalized):
		return modernTokenizerEncodingName
	case strings.HasPrefix(normalized, "chatgpt"):
		return modernTokenizerEncodingName
	case strings.HasPrefix(normalized, "text-embedding-"):
		return defaultTokenizerEncodingName
	default:
		return ""
	}
}

func isOReasoningModelName(model string) bool {
	normalized := strings.TrimSpace(strings.ToLower(model))
	if len(normalized) < 2 || normalized[0] != 'o' || !unicode.IsDigit(rune(normalized[1])) {
		return false
	}
	if len(normalized) == 2 {
		return true
	}
	next := normalized[2]
	return next == '-' || next == '_' || next == '.'
}

func getEncodingByName(name string) (*tiktoken.Tiktoken, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("encoding name is empty")
	}
	return tiktoken.GetEncoding(name)
}

func resolveTokenEncoder(model string) (*tiktoken.Tiktoken, string, error) {
	if encodingName := resolveTokenizerEncodingName(model); encodingName != "" {
		tokenEncoder, err := getEncodingByName(encodingName)
		if err != nil {
			return nil, encodingName, err
		}
		return tokenEncoder, encodingName, nil
	}
	tokenEncoder, err := tiktoken.EncodingForModel(model)
	if err != nil {
		return nil, "", err
	}
	return tokenEncoder, "", nil
}

func ensureDefaultTokenEncoder() *tiktoken.Tiktoken {
	tokenEncoderMu.RLock()
	if defaultTokenEncoder != nil {
		defer tokenEncoderMu.RUnlock()
		return defaultTokenEncoder
	}
	tokenEncoderMu.RUnlock()

	tokenEncoderMu.Lock()
	defer tokenEncoderMu.Unlock()
	if defaultTokenEncoder != nil {
		return defaultTokenEncoder
	}
	tokenEncoder, err := getEncodingByName(defaultTokenizerEncodingName)
	if err != nil {
		logger.SysError("failed to lazily initialize default token encoder: " + err.Error())
		return nil
	}
	defaultTokenEncoder = tokenEncoder
	tokenEncoderMap[defaultTokenizerEncodingName] = tokenEncoder
	return defaultTokenEncoder
}

func InitTokenEncoders() {
	logger.SysLog("initializing token encoders")
	defaultEncoder, err := getEncodingByName(defaultTokenizerEncodingName)
	if err != nil {
		logger.FatalLog(fmt.Sprintf("failed to get %s token encoder: %s, "+
			"if you are using in offline environment, please set TIKTOKEN_CACHE_DIR to use exsited files, check this link for more information: https://stackoverflow.com/questions/76106366/how-to-use-tiktoken-in-offline-mode-computer ", defaultTokenizerEncodingName, err.Error()))
	}
	modernEncoder, err := getEncodingByName(modernTokenizerEncodingName)
	if err != nil {
		logger.FatalLog(fmt.Sprintf("failed to get %s token encoder: %s", modernTokenizerEncodingName, err.Error()))
	}
	tokenEncoderMu.Lock()
	defaultTokenEncoder = defaultEncoder
	tokenEncoderMap[defaultTokenizerEncodingName] = defaultEncoder
	tokenEncoderMap[modernTokenizerEncodingName] = modernEncoder
	tokenEncoderMu.Unlock()
	logger.SysLog("token encoders initialized")
}

func getTokenEncoder(model string) *tiktoken.Tiktoken {
	defaultEncoder := ensureDefaultTokenEncoder()
	cacheKey := strings.TrimSpace(strings.ToLower(model))
	if cacheKey == "" {
		cacheKey = defaultTokenizerEncodingName
	}
	tokenEncoderMu.RLock()
	tokenEncoder, ok := tokenEncoderMap[cacheKey]
	if ok && tokenEncoder != nil {
		tokenEncoderMu.RUnlock()
		return tokenEncoder
	}
	tokenEncoderMu.RUnlock()
	if ok {
		tokenEncoder, encodingName, err := resolveTokenEncoder(model)
		if err != nil {
			if encodingName != "" {
				logger.SysWarnf("[tokenizer] encoder_fallback model=%q preferred_encoding=%q fallback=%q err=%q", model, encodingName, "cl100k_base", err.Error())
			} else {
				logger.SysWarnf("[tokenizer] encoder_fallback model=%q fallback=%q err=%q", model, "cl100k_base", err.Error())
			}
			tokenEncoder = defaultEncoder
		}
		tokenEncoderMu.Lock()
		tokenEncoderMap[cacheKey] = tokenEncoder
		tokenEncoderMu.Unlock()
		return tokenEncoder
	}
	tokenEncoder, encodingName, err := resolveTokenEncoder(model)
	if err != nil {
		if encodingName != "" {
			logger.SysWarnf("[tokenizer] encoder_fallback model=%q preferred_encoding=%q fallback=%q err=%q", model, encodingName, "cl100k_base", err.Error())
		} else {
			logger.SysWarnf("[tokenizer] encoder_fallback model=%q fallback=%q err=%q", model, "cl100k_base", err.Error())
		}
		tokenEncoder = defaultEncoder
	}
	tokenEncoderMu.Lock()
	tokenEncoderMap[cacheKey] = tokenEncoder
	tokenEncoderMu.Unlock()
	return tokenEncoder
}

func getTokenNum(tokenEncoder *tiktoken.Tiktoken, text string) int {
	if tokenEncoder == nil {
		return 0
	}
	return len(tokenEncoder.Encode(text, nil, nil))
}

func CountTokenMessages(messages []model.Message, model string) int {
	tokenEncoder := getTokenEncoder(model)
	// Reference:
	//
	// Every message follows <|start|>{role/name}\n{content}<|end|>\n
	var tokensPerMessage int
	var tokensPerName int
	if model == "gpt-3.5-turbo-0301" {
		tokensPerMessage = 4
		tokensPerName = -1 // If there's a name, the role is omitted
	} else {
		tokensPerMessage = 3
		tokensPerName = 1
	}
	tokenNum := 0
	for _, message := range messages {
		tokenNum += tokensPerMessage
		switch v := message.Content.(type) {
		case string:
			tokenNum += getTokenNum(tokenEncoder, v)
		case []any:
			for _, it := range v {
				m := it.(map[string]any)
				switch m["type"] {
				case "text":
					if textValue, ok := m["text"]; ok {
						if textString, ok := textValue.(string); ok {
							tokenNum += getTokenNum(tokenEncoder, textString)
						}
					}
				case "image_url":
					imageUrl, ok := m["image_url"].(map[string]any)
					if ok {
						url := imageUrl["url"].(string)
						detail := ""
						if imageUrl["detail"] != nil {
							detail = imageUrl["detail"].(string)
						}
						imageTokens, err := countImageTokens(url, detail, model)
						if err != nil {
							logger.SysError("error counting image tokens: " + err.Error())
						} else {
							tokenNum += imageTokens
						}
					}
				}
			}
		}
		tokenNum += getTokenNum(tokenEncoder, message.Role)
		if message.Name != nil {
			tokenNum += tokensPerName
			tokenNum += getTokenNum(tokenEncoder, *message.Name)
		}
	}
	tokenNum += 3 // Every reply is primed with <|start|>assistant<|message|>
	return tokenNum
}

const (
	lowDetailCost         = 85
	highDetailCostPerTile = 170
	additionalCost        = 85
	// gpt-4o-mini cost higher than other model
	gpt4oMiniLowDetailCost  = 2833
	gpt4oMiniHighDetailCost = 5667
	gpt4oMiniAdditionalCost = 2833
)

func countImageTokens(url string, detail string, model string) (_ int, err error) {
	var fetchSize = true
	var width, height int
	// detail == "auto" is undocumented on how it works, it just said the model will use the auto setting which will look at the image input size and decide if it should use the low or high setting.
	// According to the official guide, "low" disable the high-res model,
	// and only receive low-res 512px x 512px version of the image, indicating
	// that image is treated as low-res when size is smaller than 512px x 512px,
	// then we can assume that image size larger than 512px x 512px is treated
	// as high-res. Then we have the following logic:
	// if detail == "" || detail == "auto" {
	// 	width, height, err = image.GetImageSize(url)
	// 	if err != nil {
	// 		return 0, err
	// 	}
	// 	fetchSize = false
	// 	// not sure if this is correct
	// 	if width > 512 || height > 512 {
	// 		detail = "high"
	// 	} else {
	// 		detail = "low"
	// 	}
	// }

	// However, in my test, it seems to be always the same as "high".
	// The following image, which is 125x50, is still treated as high-res, taken
	// 255 tokens in the response of non-stream chat completion api.
	if detail == "" || detail == "auto" {
		// assume by test, not sure if this is correct
		detail = "high"
	}
	switch detail {
	case "low":
		if strings.HasPrefix(model, "gpt-4o-mini") {
			return gpt4oMiniLowDetailCost, nil
		}
		return lowDetailCost, nil
	case "high":
		if fetchSize {
			width, height, err = image.GetImageSize(url)
			if err != nil {
				return 0, err
			}
		}
		if width > 2048 || height > 2048 { // max(width, height) > 2048
			ratio := float64(2048) / math.Max(float64(width), float64(height))
			width = int(float64(width) * ratio)
			height = int(float64(height) * ratio)
		}
		if width > 768 && height > 768 { // min(width, height) > 768
			ratio := float64(768) / math.Min(float64(width), float64(height))
			width = int(float64(width) * ratio)
			height = int(float64(height) * ratio)
		}
		numSquares := int(math.Ceil(float64(width)/512) * math.Ceil(float64(height)/512))
		if strings.HasPrefix(model, "gpt-4o-mini") {
			return numSquares*gpt4oMiniHighDetailCost + gpt4oMiniAdditionalCost, nil
		}
		result := numSquares*highDetailCostPerTile + additionalCost
		return result, nil
	default:
		return 0, errors.New("invalid detail option")
	}
}

func CountTokenInput(input any, model string) int {
	switch v := input.(type) {
	case string:
		return CountTokenText(v, model)
	case []string:
		text := ""
		for _, s := range v {
			text += s
		}
		return CountTokenText(text, model)
	}
	return 0
}

func CountTokenText(text string, model string) int {
	tokenEncoder := getTokenEncoder(model)
	return getTokenNum(tokenEncoder, text)
}

func CountToken(text string) int {
	return CountTokenInput(text, defaultTokenizerEncodingName)
}
