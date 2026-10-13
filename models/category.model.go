package models

import (
	"time"

	"gorm.io/gorm"
)

type Category struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	BeautyCenterID uint `gorm:"not null" json:"beauty_center_id"`

	Name        string `gorm:"size:100;not null" json:"name"`
	Description string `json:"description"`
	// Per-locale overrides; reads resolve by `lang` with fallback to the base.
	NameI18n        map[string]string `gorm:"serializer:json" json:"name_i18n,omitempty"`
	DescriptionI18n map[string]string `gorm:"serializer:json" json:"description_i18n,omitempty"`
	Icon        string `gorm:"size:10" json:"icon"`

	Services []Service `gorm:"foreignKey:CategoryID" json:"services"`
}
