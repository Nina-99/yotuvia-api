package tests

import (
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// completeCenterProfile satisfies the verification checklist: name, address,
// phone, description + at least one schedule and one image.
func completeCenterProfile(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()
	require.NoError(t, db.Model(&models.BeautyCenter{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"address":     "Calle 1 #123",
			"description": "Salón de prueba",
		}).Error)
	require.NoError(t, db.Create(&models.WorkingHour{
		BeautyCenterID: id,
		DayOfWeek:      1,
		StartTime:      "09:00",
		EndTime:        "18:00",
	}).Error)
	require.NoError(t, db.Create(&models.BeautyCenterImage{
		BeautyCenterID: id,
		ImageUrl:       "https://cdn.example.com/center.jpg",
		IsProfile:      true,
	}).Error)
}
