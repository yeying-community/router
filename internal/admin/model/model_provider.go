package model

// ModelProvider stores model provider catalog in a dedicated table.
type ModelProvider struct {
	Provider  string `json:"provider" gorm:"primaryKey;type:varchar(64)"`
	Name      string `json:"name" gorm:"type:varchar(128);default:''"`
	Models    string `json:"models" gorm:"type:text"`
	BaseURL   string `json:"base_url" gorm:"column:base_url;type:text"`
	APIKey    string `json:"api_key" gorm:"column:api_key;type:text"`
	Source    string `json:"source" gorm:"type:varchar(32);default:'manual'"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint"`
}

func (ModelProvider) TableName() string {
	return "model_providers"
}
