package model

const (
	ProviderModelsTableName = "provider_models"
)

type ProviderModel struct {
	Provider           string  `json:"provider" gorm:"primaryKey;type:varchar(64)"`
	Model              string  `json:"model" gorm:"primaryKey;type:varchar(255)"`
	Type               string  `json:"type" gorm:"type:varchar(32);default:'text'"`
	Description        string  `json:"description" gorm:"type:text;default:''"`
	IsDeleted          bool    `json:"is_deleted" gorm:"not null;default:false"`
	SupportedEndpoints string  `json:"supported_endpoints" gorm:"type:text;default:''"`
	InputPrice         float64 `json:"input_price" gorm:"type:double precision;default:0"`
	OutputPrice        float64 `json:"output_price" gorm:"type:double precision;default:0"`
	PriceUnit          string  `json:"price_unit" gorm:"type:varchar(64);default:'per_1k_tokens'"`
	Currency           string  `json:"currency" gorm:"type:varchar(16);default:'USD'"`
	Source             string  `json:"source" gorm:"type:varchar(32);default:'manual'"`
	UpdatedAt          int64   `json:"updated_at" gorm:"bigint"`
}

func (ProviderModel) TableName() string {
	return ProviderModelsTableName
}
