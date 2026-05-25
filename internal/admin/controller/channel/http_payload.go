package channel

import (
	"encoding/json"
	"net/http"
	"strings"
)

func maskSecretValue(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(normalized), "bearer ") {
		return "Bearer ***"
	}
	return "***"
}

func sanitizeHTTPHeadersForLog(header http.Header) map[string]string {
	if len(header) == 0 {
		return map[string]string{}
	}
	result := make(map[string]string, len(header))
	for key, values := range header {
		if len(values) == 0 {
			result[key] = ""
			continue
		}
		value := values[0]
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "authorization", "api-key", "x-api-key":
			result[key] = maskSecretValue(value)
		default:
			result[key] = value
		}
	}
	return result
}

func marshalJSONForLog(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func buildHTTPRequestPayloadForLog(method string, url string, header http.Header, body []byte) string {
	payload := map[string]any{
		"method":  strings.TrimSpace(method),
		"url":     strings.TrimSpace(url),
		"headers": sanitizeHTTPHeadersForLog(header),
	}
	for key, value := range encodeHTTPBodyForLog(body, header.Get("Content-Type")) {
		payload[key] = value
	}
	return marshalJSONForLog(payload)
}

func buildHTTPResponsePayloadForLog(statusCode int, header http.Header, body []byte) string {
	payload := map[string]any{
		"status_code": statusCode,
		"headers":     sanitizeHTTPHeadersForLog(header),
	}
	for key, value := range encodeHTTPBodyForLog(body, header.Get("Content-Type")) {
		payload[key] = value
	}
	return marshalJSONForLog(payload)
}
