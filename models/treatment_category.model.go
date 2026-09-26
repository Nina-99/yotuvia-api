package models

import (
	"time"

	"gorm.io/gorm"
)

// TreatmentCategory is the GLOBAL, center-agnostic grouping of the shared
// treatment vocabulary. Unlike Category (which is owned by one beauty center),
// it has no BeautyCenterID: "Peluquería" means the same thing for every center,
// which is exactly what makes a cross-center "find me a haircut" search honest.
type TreatmentCategory struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	Name        string `gorm:"size:100;not null" json:"name"`
	Description string `json:"description"`
	Icon        string `gorm:"size:10" json:"icon"`

	Treatments []Treatment `gorm:"foreignKey:TreatmentCategoryID" json:"treatments,omitempty"`
}
