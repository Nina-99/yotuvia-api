package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/configs"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// taxonomyTestDB builds the standard test schema and points the global seeds at
// it (same pattern as token_seed_heal_test.go).
func taxonomyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	configs.DB = db
	t.Cleanup(func() { configs.DB = nil })
	return db
}

// TestSeedTreatmentTaxonomy_Idempotent proves a second boot creates no duplicate
// categories or treatments.
func TestSeedTreatmentTaxonomy_Idempotent(t *testing.T) {
	db := taxonomyTestDB(t)

	configs.SeedTreatmentTaxonomy()

	var cats1, treats1 int64
	require.NoError(t, db.Model(&models.TreatmentCategory{}).Count(&cats1).Error)
	require.NoError(t, db.Model(&models.Treatment{}).Count(&treats1).Error)
	require.Greater(t, cats1, int64(0))
	require.Greater(t, treats1, int64(0))

	configs.SeedTreatmentTaxonomy()

	var cats2, treats2 int64
	require.NoError(t, db.Model(&models.TreatmentCategory{}).Count(&cats2).Error)
	require.NoError(t, db.Model(&models.Treatment{}).Count(&treats2).Error)
	assert.Equal(t, cats1, cats2, "second seed must not duplicate categories")
	assert.Equal(t, treats1, treats2, "second seed must not duplicate treatments")
}

// TestBackfillServiceTreatments_NormalizedAndNonOverwrite proves the backfill
// matches case/accent-insensitively, never overwrites an existing link, and
// reports the match rate plus the distinct unmatched names.
func TestBackfillServiceTreatments_NormalizedAndNonOverwrite(t *testing.T) {
	db := taxonomyTestDB(t)
	configs.SeedTreatmentTaxonomy()

	var corte, mani, nail models.Treatment
	require.NoError(t, db.Where("name = ?", "Corte clásico").First(&corte).Error)
	require.NoError(t, db.Where("name = ?", "Manicuría semipermanente").First(&mani).Error)
	require.NoError(t, db.Where("name = ?", "Nail art").First(&nail).Error)

	mk := func(id uint, name string, treatmentID *uint) {
		t.Helper()
		require.NoError(t, db.Create(&models.Service{
			ID: id, BeautyCenterID: 1, CategoryID: 1, Name: name,
			Description: "d", Duration: 30, Price: 10, IsActive: true, TreatmentID: treatmentID,
		}).Error)
	}
	mk(1, "  CORTE CLASICO  ", nil)        // case + surrounding whitespace
	mk(2, "corte clásico", nil)            // lowercase + accent
	mk(3, "MANICURÍA SEMIPERMANENTE", nil) // uppercase + accents
	preLinked := nail.ID
	mk(4, "Corte clásico", &preLinked)  // already linked → must NOT be overwritten
	mk(5, "Unknown Fancy Service", nil) // unmatched

	stats := configs.BackfillServiceTreatments()

	assert.Equal(t, 5, stats.TotalServices)
	assert.Equal(t, 4, stats.Eligible)
	assert.Equal(t, 3, stats.Linked)
	assert.Equal(t, 1, stats.AlreadyLinked)
	assert.InDelta(t, 75.0, stats.MatchRate, 0.01)
	assert.Equal(t, []string{"Unknown Fancy Service"}, stats.UnmatchedNames)

	var s1, s2, s3, s4, s5 models.Service
	require.NoError(t, db.First(&s1, 1).Error)
	require.NoError(t, db.First(&s2, 2).Error)
	require.NoError(t, db.First(&s3, 3).Error)
	require.NoError(t, db.First(&s4, 4).Error)
	require.NoError(t, db.First(&s5, 5).Error)

	require.NotNil(t, s1.TreatmentID)
	require.NotNil(t, s2.TreatmentID)
	require.NotNil(t, s3.TreatmentID)
	assert.Equal(t, corte.ID, *s1.TreatmentID)
	assert.Equal(t, corte.ID, *s2.TreatmentID)
	assert.Equal(t, mani.ID, *s3.TreatmentID)

	// Pre-existing link is untouched even though the name matches a treatment.
	require.NotNil(t, s4.TreatmentID)
	assert.Equal(t, nail.ID, *s4.TreatmentID, "backfill must never overwrite a non-null TreatmentID")
	assert.Nil(t, s5.TreatmentID, "unmatched services stay unlinked")

	// Second run: nothing new is linked, and no existing link is disturbed. The
	// still-unmatched row stays eligible (and still unmatched).
	stats2 := configs.BackfillServiceTreatments()
	assert.Equal(t, 1, stats2.Eligible)
	assert.Equal(t, 0, stats2.Linked)
	assert.Equal(t, []string{"Unknown Fancy Service"}, stats2.UnmatchedNames)
}

// TestTreatmentCatalogEndpoint proves GET /api/treatments returns the grouped
// global catalog without authentication.
func TestTreatmentCatalogEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := taxonomyTestDB(t)
	configs.SeedTreatmentTaxonomy()

	var activeTreatments int64
	require.NoError(t, db.Model(&models.Treatment{}).Where("is_active = ?", true).Count(&activeTreatments).Error)

	r := gin.New()
	router.TreatmentRoutes(r.Group("/api"), db, validator.New())

	w := do(r, http.MethodGet, "/api/treatments", "", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body response.TreatmentCatalogResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotEmpty(t, body.Categories)
	assert.Equal(t, int(activeTreatments), body.Total)

	found := false
	for _, c := range body.Categories {
		for _, tr := range c.Treatments {
			if tr.Name == "Corte clásico" {
				found = true
			}
		}
	}
	assert.True(t, found, "catalog must include the canonical 'Corte clásico'")
}
