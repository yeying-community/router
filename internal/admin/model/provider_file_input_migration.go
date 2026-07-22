package model

import (
	"strings"

	"gorm.io/gorm"
)

type providerFileInputCatalogEntry struct {
	provider  string
	models    []string
	endpoint  string
	fileTypes []string
	upload    bool
	url       bool
}

// refreshOfficialProviderFileInputWithDB marks only catalogued models with a
// documented provider-native file input API. Channel PDF policy alone is not
// evidence that every model on that channel understands files.
func refreshOfficialProviderFileInputWithDB(db *gorm.DB) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	entries := []providerFileInputCatalogEntry{
		{provider: "openai", models: []string{"gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano", "gpt-4o", "gpt-4o-mini", "gpt-5", "gpt-5-mini", "gpt-5-nano", "gpt-5-pro", "gpt-5.1", "gpt-5.2", "gpt-5.4", "gpt-5.4-mini", "gpt-5.4-nano", "gpt-5.4-pro", "gpt-5.5", "gpt-5.5-pro"}, endpoint: "/v1/responses", fileTypes: []string{"pdf", "docx", "pptx", "xlsx", "txt", "csv"}, upload: true, url: true},
		{provider: "anthropic", models: []string{"claude-3-5-haiku-20241022", "claude-haiku-4-5", "claude-haiku-4-5-20251001", "claude-opus-4-1", "claude-opus-4-1-20250805", "claude-opus-4-5", "claude-opus-4-5-20251101", "claude-opus-4-6", "claude-opus-4-6-thinking", "claude-opus-4-7", "claude-opus-4-8", "claude-sonnet-4-5", "claude-sonnet-4-5-20250929", "claude-sonnet-4-6"}, endpoint: "/v1/messages", fileTypes: []string{"pdf"}, upload: true, url: true},
		{provider: "google", models: []string{"gemini-2.5-flash", "gemini-2.5-flash-lite", "gemini-2.5-pro"}, endpoint: "/v1/chat/completions", fileTypes: []string{"pdf", "docx", "pptx", "xlsx", "txt", "csv", "mp3", "wav", "mp4"}, upload: true, url: true},
	}
	for _, entry := range entries {
		for _, modelName := range entry.models {
			var row ProviderModel
			if err := db.Where("provider = ? AND model = ?", entry.provider, modelName).First(&row).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					continue
				}
				return err
			}
			spec, err := ParseProviderModelSpecification(row.Specification)
			if err != nil {
				return err
			}
			if spec == nil {
				spec = &ProviderModelSpecification{Version: 1}
			}
			if spec.Endpoints == nil {
				spec.Endpoints = map[string]ProviderModelEndpointSpecification{}
			}
			endpoint := spec.Endpoints[entry.endpoint]
			endpoint.InputModalities = normalizeSpecificationValues(append(endpoint.InputModalities, "file"))
			endpoint.FileTypes = normalizeSpecificationValues(append(endpoint.FileTypes, entry.fileTypes...))
			endpoint.SupportsUpload = endpoint.SupportsUpload || entry.upload
			endpoint.SupportsURL = endpoint.SupportsURL || entry.url
			spec.Endpoints[entry.endpoint] = endpoint
			tags := strings.Join(NormalizeProviderModelTags(append(splitProviderModelTags(row.Tags), ProviderModelTagFileInput)), ",")
			if err := db.Model(&ProviderModel{}).Where("provider = ? AND model = ?", entry.provider, modelName).Updates(map[string]any{"tags": tags, "specification": MarshalProviderModelSpecification(spec)}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
