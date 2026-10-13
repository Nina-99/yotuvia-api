package models

import (
	"time"

	"gorm.io/gorm"
)

type Service struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	BeautyCenterID uint     `gorm:"not null;index" json:"beauty_center_id"`
	CategoryID     uint     `gorm:"not null;index" json:"category_id"`
	Category       Category `gorm:"foreignKey:CategoryID" json:"category,omitempty"`

	Name        string  `gorm:"size:100;not null" json:"name"`
	Description string  `gorm:"not null" json:"description"`
	// Per-locale overrides ({"es": "...", "en": "..."}). Name/Description stay
	// the canonical fallback; reads resolve by the requested `lang`.
	NameI18n        map[string]string `gorm:"serializer:json" json:"name_i18n,omitempty"`
	DescriptionI18n map[string]string `gorm:"serializer:json" json:"description_i18n,omitempty"`
	Duration    int     `gorm:"not null" json:"duration"`
	Price       float64 `json:"price"`
	ImageURL    string  `json:"image_url"`
	IsActive    bool    `gorm:"default:true" json:"is_active"`

	// TreatmentID links this per-center offering to the GLOBAL treatment
	// vocabulary. It is NULLABLE on purpose: existing rows keep working and a
	// center may offer an uncatalogued custom service. Price and Duration stay
	// per-center — the global part is only the vocabulary.
	TreatmentID *uint      `gorm:"index" json:"treatment_id,omitempty"`
	Treatment   *Treatment `gorm:"foreignKey:TreatmentID" json:"treatment,omitempty"`
}
