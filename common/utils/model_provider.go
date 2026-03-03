package utils

import "strings"

// NormalizeModelProvider canonicalizes provider aliases for filtering and persistence.
func NormalizeModelProvider(provider string) string {
	trimmed := strings.TrimSpace(provider)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	switch lower {
	case "gpt", "openai":
		return "openai"
	case "gemini", "google":
		return "google"
	case "claude", "anthropic":
		return "anthropic"
	case "deepseek":
		return "deepseek"
	case "qwen", "qwq", "qvq", "千问":
		return "qwen"
	default:
		return lower
	}
}

// ResolveModelProvider infers provider from model naming rules to keep backend and frontend consistent.
func ResolveModelProvider(modelName string) string {
	name := strings.TrimSpace(modelName)
	if name == "" {
		return "unknown"
	}
	if strings.Contains(name, "/") {
		parts := strings.SplitN(name, "/", 2)
		prefix := NormalizeModelProvider(parts[0])
		if prefix == "" {
			return "unknown"
		}
		return prefix
	}
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "gpt-"),
		strings.HasPrefix(lower, "o1"),
		strings.HasPrefix(lower, "o3"),
		strings.HasPrefix(lower, "chatgpt-"):
		return "openai"
	case strings.HasPrefix(lower, "claude-"):
		return "anthropic"
	case strings.HasPrefix(lower, "gemini-"):
		return "google"
	case strings.HasPrefix(lower, "deepseek-"):
		return "deepseek"
	case strings.HasPrefix(lower, "qwen-"),
		strings.HasPrefix(lower, "qwq-"),
		strings.HasPrefix(lower, "qvq-"):
		return "qwen"
	case strings.HasPrefix(lower, "glm-"),
		strings.HasPrefix(lower, "cogview-"):
		return "zhipu"
	case strings.HasPrefix(lower, "ernie-"):
		return "baidu"
	default:
		return "unknown"
	}
}

// ResolveOwnedByProvider infers provider from OpenAI-compatible `owned_by`.
func ResolveOwnedByProvider(ownedBy string) string {
	value := strings.TrimSpace(strings.ToLower(ownedBy))
	if value == "" {
		return "unknown"
	}
	canonical := NormalizeModelProvider(value)
	if canonical != value {
		return canonical
	}
	switch {
	case strings.Contains(value, "openai"),
		strings.Contains(value, "gpt"):
		return "openai"
	case strings.Contains(value, "anthropic"),
		strings.Contains(value, "claude"):
		return "anthropic"
	case strings.Contains(value, "google"),
		strings.Contains(value, "gemini"):
		return "google"
	case strings.Contains(value, "deepseek"):
		return "deepseek"
	case strings.Contains(value, "qwen"),
		strings.Contains(value, "qwq"),
		strings.Contains(value, "qvq"):
		return "qwen"
	default:
		return value
	}
}

// MatchModelProvider matches a model/provider metadata pair to the provider filter.
func MatchModelProvider(modelName string, ownedBy string, provider string) bool {
	filter := NormalizeModelProvider(provider)
	if filter == "" {
		return true
	}
	if ResolveModelProvider(modelName) == filter {
		return true
	}
	if ResolveOwnedByProvider(ownedBy) == filter {
		return true
	}
	lowerName := strings.ToLower(modelName)
	lowerOwnedBy := strings.ToLower(ownedBy)
	return strings.Contains(lowerName, filter) || strings.Contains(lowerOwnedBy, filter)
}
