package model

import (
	"encoding/json"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type qwenChinaPriceComponent struct {
	model     string
	component string
	input     float64
	output    float64
	unit      string
}

// refreshQwenChinaPricingWithDB fills prices that previously existed as zero-valued
// realtime Qwen catalog rows. Prices are China mainland list prices from Alibaba
// Model Studio and are stored as separate realtime text/audio components.
func refreshQwenChinaPricingWithDB(db *gorm.DB) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	const sourceURL = "https://help.aliyun.com/zh/model-studio/model-pricing"
	fileTranscriptionEndpoints, _ := json.Marshal([]string{"/v1/audio/transcriptions"})
	fileTranscriptionTags := strings.Join(NormalizeProviderModelTags([]string{ProviderModelTagAudio, ProviderModelTagFileInput}), ",")
	fileTranscriptionSpec, _ := json.Marshal(ProviderModelSpecification{
		Version: 1,
		Endpoints: map[string]ProviderModelEndpointSpecification{
			"/v1/audio/transcriptions": {
				InputModalities: []string{"audio_file"},
				FileTypes:       []string{"mp3", "wav", "m4a", "flac", "aac"},
				SupportsUpload:  true,
			},
		},
	})
	if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider"}, {Name: "model"}}, DoUpdates: clause.AssignmentColumns([]string{"tags", "supported_endpoints", "specification", "input_price", "output_price", "price_unit", "currency", "source", "updated_at"})}).Create(&ProviderModel{
		Provider: "qwen", Model: "qwen3-asr-flash-filetrans", Tags: fileTranscriptionTags,
		SupportedEndpoints: string(fileTranscriptionEndpoints), Specification: string(fileTranscriptionSpec), InputPrice: 0.00022, PriceUnit: ProviderPriceUnitPerSecond,
		Currency: "CNY", Source: "migration", UpdatedAt: helper.GetTimestamp(),
	}).Error; err != nil {
		return err
	}
	omniEndpoints, _ := json.Marshal([]string{"/v1/chat/completions"})
	omniTags := strings.Join(NormalizeProviderModelTags([]string{ProviderModelTagAudio, ProviderModelTagVision, ProviderModelTagToolCalling}), ",")
	for _, omni := range []struct {
		model                              string
		textIn, textOut, audioIn, audioOut float64
	}{
		{model: "qwen3.5-omni-plus", textIn: 7, textOut: 53, audioIn: 40, audioOut: 213},
		{model: "qwen3.5-omni-flash", textIn: 2.2, textOut: 18, audioIn: 13.3, audioOut: 72},
	} {
		if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider"}, {Name: "model"}}, DoUpdates: clause.AssignmentColumns([]string{"tags", "supported_endpoints", "input_price", "output_price", "price_unit", "currency", "source", "updated_at"})}).Create(&ProviderModel{
			Provider: "qwen", Model: omni.model, Tags: omniTags, SupportedEndpoints: string(omniEndpoints),
			InputPrice: omni.textIn, OutputPrice: omni.textOut, PriceUnit: ProviderPriceUnitPer1KTokens,
			Currency: "CNY", Source: "migration", UpdatedAt: helper.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		for _, component := range []ProviderModelPriceComponent{
			{Provider: "qwen", Model: omni.model, Component: ProviderModelPriceComponentRealtimeText, InputPrice: omni.textIn, OutputPrice: omni.textOut, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "migration", SourceURL: sourceURL, SortOrder: 10, UpdatedAt: helper.GetTimestamp()},
			{Provider: "qwen", Model: omni.model, Component: ProviderModelPriceComponentRealtimeAudio, InputPrice: omni.audioIn, OutputPrice: omni.audioOut, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "migration", SourceURL: sourceURL, SortOrder: 20, UpdatedAt: helper.GetTimestamp()},
		} {
			if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider"}, {Name: "model"}, {Name: "component"}, {Name: "condition"}}, DoUpdates: clause.AssignmentColumns([]string{"input_price", "output_price", "price_unit", "currency", "source", "source_url", "sort_order", "updated_at"})}).Create(&component).Error; err != nil {
				return err
			}
		}
	}
	rows := []qwenChinaPriceComponent{
		{model: "qwen3.5-omni-plus-realtime", component: ProviderModelPriceComponentRealtimeText, input: 10, output: 80, unit: ProviderPriceUnitPer1KTokens},
		{model: "qwen3.5-omni-plus-realtime", component: ProviderModelPriceComponentRealtimeAudio, input: 60, output: 300, unit: ProviderPriceUnitPer1KTokens},
		{model: "qwen3.5-omni-flash-realtime", component: ProviderModelPriceComponentRealtimeText, input: 3.3, output: 27, unit: ProviderPriceUnitPer1KTokens},
		{model: "qwen3.5-omni-flash-realtime", component: ProviderModelPriceComponentRealtimeAudio, input: 20, output: 107, unit: ProviderPriceUnitPer1KTokens},
		{model: "qwen3.5-livetranslate-flash-realtime", component: ProviderModelPriceComponentRealtimeText, input: 3.3, output: 160, unit: ProviderPriceUnitPer1KTokens},
		{model: "qwen3.5-livetranslate-flash-realtime", component: ProviderModelPriceComponentRealtimeAudio, input: 40, output: 100, unit: ProviderPriceUnitPer1KTokens},
		{model: "qwen-tts-realtime", component: ProviderModelPriceComponentRealtimeText, input: 2.4, output: 12, unit: ProviderPriceUnitPer1KTokens},
		{model: "qwen3-tts-flash-realtime", component: ProviderModelPriceComponentRealtimeText, input: 0.1, output: 0, unit: ProviderPriceUnitPer1KChars},
		{model: "qwen3-tts-instruct-flash-realtime", component: ProviderModelPriceComponentRealtimeText, input: 0.1, output: 0, unit: ProviderPriceUnitPer1KChars},
	}
	for _, row := range rows {
		component := ProviderModelPriceComponent{
			Provider: "qwen", Model: row.model, Component: row.component,
			InputPrice: row.input, OutputPrice: row.output, PriceUnit: row.unit,
			Currency: "CNY", Source: "migration", SourceURL: sourceURL,
			SortOrder: 10, UpdatedAt: helper.GetTimestamp(),
		}
		if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider"}, {Name: "model"}, {Name: "component"}, {Name: "condition"}}, DoUpdates: clause.AssignmentColumns([]string{"input_price", "output_price", "price_unit", "currency", "source", "source_url", "sort_order", "updated_at"})}).Create(&component).Error; err != nil {
			return err
		}
	}
	// File-transfer models are file inputs, not vision models. Apply the tag only
	// when a provider catalog row already exists; catalog insertion is separate.
	var models []ProviderModel
	if err := db.Where("provider = ? AND (model LIKE ? OR model LIKE ?)", "qwen", "%filetrans%", "%file-trans%").Find(&models).Error; err != nil {
		return err
	}
	for _, model := range models {
		tags := NormalizeProviderModelTags(append(splitProviderModelTags(model.Tags), ProviderModelTagFileInput))
		if err := db.Model(&ProviderModel{}).Where("provider = ? AND model = ?", model.Provider, model.Model).Update("tags", strings.Join(tags, ",")).Error; err != nil {
			return err
		}
	}
	vlEndpoints, _ := json.Marshal([]string{"/v1/chat/completions"})
	for _, vl := range []struct {
		model         string
		input, output float64
		tags          []string
	}{
		{model: "qwen3-vl-235b-a22b-thinking", input: 2, output: 20, tags: []string{ProviderModelTagVision, ProviderModelTagReasoning, ProviderModelTagToolCalling}},
		{model: "qwen3-vl-235b-a22b-instruct", input: 2, output: 8, tags: []string{ProviderModelTagVision, ProviderModelTagToolCalling}},
		{model: "qwen3-vl-32b-thinking", input: 2, output: 20, tags: []string{ProviderModelTagVision, ProviderModelTagReasoning, ProviderModelTagToolCalling}},
		{model: "qwen3-vl-32b-instruct", input: 2, output: 8, tags: []string{ProviderModelTagVision, ProviderModelTagToolCalling}},
		{model: "qwen3-vl-30b-a3b-thinking", input: 0.75, output: 7.5, tags: []string{ProviderModelTagVision, ProviderModelTagReasoning, ProviderModelTagToolCalling}},
		{model: "qwen3-vl-30b-a3b-instruct", input: 0.75, output: 3, tags: []string{ProviderModelTagVision, ProviderModelTagToolCalling}},
		{model: "qwen3-vl-8b-thinking", input: 0.5, output: 5, tags: []string{ProviderModelTagVision, ProviderModelTagReasoning, ProviderModelTagToolCalling}},
		{model: "qwen3-vl-8b-instruct", input: 0.5, output: 2, tags: []string{ProviderModelTagVision, ProviderModelTagToolCalling}},
	} {
		if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider"}, {Name: "model"}}, DoUpdates: clause.AssignmentColumns([]string{"tags", "supported_endpoints", "input_price", "output_price", "price_unit", "currency", "source", "updated_at"})}).Create(&ProviderModel{
			Provider: "qwen", Model: vl.model, Tags: strings.Join(NormalizeProviderModelTags(append([]string{ProviderModelTagText}, vl.tags...)), ","), SupportedEndpoints: string(vlEndpoints),
			InputPrice: vl.input, OutputPrice: vl.output, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "migration", UpdatedAt: helper.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
	}
	audioEndpoints, _ := json.Marshal([]string{"/v1/audio/transcriptions"})
	for _, audio := range []struct {
		model, endpoint string
		input, output   float64
		unit            string
		tags            []string
	}{
		{model: "qwen3-tts-flash", endpoint: "/v1/audio/speech", input: 0.08, unit: ProviderPriceUnitPer1KChars, tags: []string{ProviderModelTagAudio}},
		{model: "qwen3-tts-instruct-flash", endpoint: "/v1/audio/speech", input: 0.08, unit: ProviderPriceUnitPer1KChars, tags: []string{ProviderModelTagAudio}},
		{model: "qwen3-tts-vd-2026-01-26", endpoint: "/v1/audio/speech", input: 0.08, unit: ProviderPriceUnitPer1KChars, tags: []string{ProviderModelTagAudio}},
		{model: "qwen3-tts-vc-2026-01-22", endpoint: "/v1/audio/speech", input: 0.08, unit: ProviderPriceUnitPer1KChars, tags: []string{ProviderModelTagAudio}},
		{model: "qwen3-asr-flash", endpoint: "/v1/audio/transcriptions", input: 0.00022, unit: ProviderPriceUnitPerSecond, tags: []string{ProviderModelTagAudio}},
		{model: "qwen3-asr-flash-realtime", endpoint: "/v1/realtime", input: 0.00033, unit: ProviderPriceUnitPerSecond, tags: []string{ProviderModelTagAudio, ProviderModelTagRealtime}},
	} {
		endpoints := audioEndpoints
		if audio.endpoint == "/v1/audio/speech" {
			endpoints, _ = json.Marshal([]string{audio.endpoint})
		} else if audio.endpoint == "/v1/realtime" {
			endpoints, _ = json.Marshal([]string{audio.endpoint})
		}
		if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider"}, {Name: "model"}}, DoUpdates: clause.AssignmentColumns([]string{"tags", "supported_endpoints", "input_price", "output_price", "price_unit", "currency", "source", "updated_at"})}).Create(&ProviderModel{
			Provider: "qwen", Model: audio.model, Tags: strings.Join(NormalizeProviderModelTags(audio.tags), ","), SupportedEndpoints: string(endpoints),
			InputPrice: audio.input, OutputPrice: audio.output, PriceUnit: audio.unit, Currency: "CNY", Source: "migration", UpdatedAt: helper.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
	}
	imageEndpoints, _ := json.Marshal([]string{"/v1/images/generations", "/v1/images/edits"})
	for _, image := range []struct {
		model string
		price float64
	}{
		{model: "qwen-image-3.0-pro", price: 0},
		{model: "qwen-image-edit-max", price: 0.5},
		{model: "qwen-image-edit-plus", price: 0.2},
		{model: "qwen-image-edit", price: 0.3},
	} {
		if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider"}, {Name: "model"}}, DoUpdates: clause.AssignmentColumns([]string{"tags", "supported_endpoints", "input_price", "output_price", "price_unit", "currency", "source", "updated_at"})}).Create(&ProviderModel{
			Provider: "qwen", Model: image.model, Tags: strings.Join(NormalizeProviderModelTags([]string{ProviderModelTagImage}), ","), SupportedEndpoints: string(imageEndpoints),
			InputPrice: image.price, PriceUnit: ProviderPriceUnitPerImage, Currency: "CNY", Source: "migration", UpdatedAt: helper.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
	}
	embeddingEndpoints, _ := json.Marshal([]string{"/v1/embeddings"})
	for _, embedding := range []struct {
		model         string
		input, output float64
	}{
		{model: "qwen3.7-text-embedding", input: 0.0005},
		{model: "qwen3-vl-embedding", input: 0.0007, output: 0.0018},
	} {
		if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider"}, {Name: "model"}}, DoUpdates: clause.AssignmentColumns([]string{"tags", "supported_endpoints", "input_price", "output_price", "price_unit", "currency", "source", "updated_at"})}).Create(&ProviderModel{
			Provider: "qwen", Model: embedding.model, Tags: ProviderModelTagEmbedding, SupportedEndpoints: string(embeddingEndpoints),
			InputPrice: embedding.input, OutputPrice: embedding.output, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "migration", UpdatedAt: helper.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
	}
	rerankEndpoints, _ := json.Marshal([]string{"/v1/rerank"})
	for _, rerank := range []struct {
		model         string
		input, output float64
	}{
		{model: "qwen3-vl-rerank", input: 0.0007, output: 0.0018},
		{model: "qwen3-rerank", input: 0.0005},
	} {
		if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider"}, {Name: "model"}}, DoUpdates: clause.AssignmentColumns([]string{"tags", "supported_endpoints", "input_price", "output_price", "price_unit", "currency", "source", "updated_at"})}).Create(&ProviderModel{
			Provider: "qwen", Model: rerank.model, Tags: ProviderModelTagText, SupportedEndpoints: string(rerankEndpoints),
			InputPrice: rerank.input, OutputPrice: rerank.output, PriceUnit: ProviderPriceUnitPer1KTokens, Currency: "CNY", Source: "migration", UpdatedAt: helper.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
