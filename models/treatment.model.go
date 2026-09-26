package models

import (
	"time"

	"gorm.io/gorm"
)

// Treatment is a GLOBAL vocabulary entry ("Corte de cabello", "Manicura").
// It is deliberately NOT tied to a center and carries NO price or duration:
// those vary per center and live on Service, which links back to the global
// vocabulary through its nullable TreatmentID. This split is why the taxonomy
// can be shared while each center keeps its own offering.
type Treatment struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	TreatmentCategoryID uint              `gorm:"not null;index" json:"treatment_category_id"`
	TreatmentCategory   TreatmentCategory `gorm:"foreignKey:TreatmentCategoryID" json:"treatment_category,omitempty"`

	// Name is the shared vocabulary term. It is unique-enough for matching:
	// the seeder is idempotent by name and the backfill matches on the
	// normalized form, so duplicate spellings never enter the catalog.
	Name     string `gorm:"size:100;not null;index" json:"name"`
	IsActive bool   `gorm:"default:true" json:"is_active"`
}
