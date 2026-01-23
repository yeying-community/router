package utils

import "strings"

// ResolveModelProvider infers provider from model naming rules to keep backend and frontend consistent.
func ResolveModelProvider(modelName string) string {
	name := strings.TrimSpace(modelName)
	if name == "" {
		return "unknown"
	}
	if strings.Contains(name, "/") {
		parts := strings.SplitN(name, "/", 2)
		prefix := strings.TrimSpace(parts[0])
		if prefix == "" {
			return "unknown"
		}
		return strings.ToLower(prefix)
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
