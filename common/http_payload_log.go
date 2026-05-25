package common

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	logPayloadPreviewLimit = 512
	logPayloadInlineLimit  = 2048
)

func payloadSHA256(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func normalizePayloadPreview(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	normalized := strings.Join(strings.Fields(trimmed), " ")
	if len(normalized) > logPayloadPreviewLimit {
		return normalized[:logPayloadPreviewLimit] + "..."
	}
	return normalized
}

func looksLikeSensitiveInlinePayload(lowerValue string) bool {
	return strings.HasPrefix(lowerValue, "data:image/") ||
		strings.HasPrefix(lowerValue, "data:audio/") ||
		strings.HasPrefix(lowerValue, "data:video/") ||
		strings.Contains(lowerValue, ";base64,")
}

func BuildPayloadLogFields(body []byte, contentType string) map[string]any {
	fields := map[string]any{
		"content_type": strings.TrimSpace(contentType),
		"body_bytes":   len(body),
	}
	if len(body) == 0 {
		fields["encoding"] = "empty"
		return fields
	}
	fields["sha256"] = payloadSHA256(body)
	lowerContentType := strings.ToLower(strings.TrimSpace(contentType))
	if utf8.Valid(body) {
		fields["encoding"] = "text"
		preview := normalizePayloadPreview(string(body))
		if preview != "" {
			fields["preview"] = preview
		}
		if strings.HasPrefix(lowerContentType, "multipart/form-data") {
			fields["preview_omitted"] = "multipart_form_data"
			delete(fields, "preview")
			return fields
		}
		if looksLikeSensitiveInlinePayload(strings.ToLower(preview)) || len(body) > logPayloadInlineLimit {
			fields["preview_truncated"] = true
		}
		return fields
	}
	fields["encoding"] = "binary"
	fields["preview_omitted"] = "binary"
	return fields
}

func MarshalPayloadLogFields(body []byte, contentType string) string {
	fields := BuildPayloadLogFields(body, contentType)
	encoded, err := json.Marshal(fields)
	if err != nil {
		return fmt.Sprintf("{\"content_type\":%q,\"body_bytes\":%d}", strings.TrimSpace(contentType), len(body))
	}
	return string(encoded)
}
