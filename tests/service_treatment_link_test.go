package tests

import (
	"testing"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The treatment link is the writer the cross-center availability search depends
// on: a service with no TreatmentID is invisible to it. The admin editor can
// therefore link AND unlink, and both directions must persist.
//
// Unlink is the subtle one: UpdateServiceRequest carries `TreatmentID *uint`, so
// "absent" and "explicit null" are indistinguishable from the JSON payload. The
// contract is a sentinel instead: treatment_id = 0 clears the link, any other
// value sets it.

func TestServiceTreatmentLink_LinksAndUnlinks(t *testing.T) {
	db := setupRouterAuthzDB(t)

	seedDashboardCenter(t, db, 1, "Center", "c@test.com", true)
	require.NoError(t, db.Create(&models.Category{
		ID: 1, BeautyCenterID: 1, Name: "Cabello",
	}).Error)
	require.NoError(t, db.Create(&models.TreatmentCategory{
		ID: 1, Name: "Peluqueria",
	}).Error)
	require.NoError(t, db.Create(&models.Treatment{
		ID: 1, TreatmentCategoryID: 1, Name: "Corte de cabello", IsActive: true,
	}).Error)
	require.NoError(t, db.Create(&models.Treatment{
		ID: 2, TreatmentCategoryID: 1, Name: "Color", IsActive: true,
	}).Error)

	svc := service.NewServiceServiceImpl(
		repository.NewServiceRepositoryImpl(db),
		repository.NewCategoryRepositoryImpl(db),
		validator.New(),
	)

	linked := uint(1)
	createdID, err := svc.Create(request.CreateServiceRequest{
		BeautyCenterID: 1,
		CategoryID:     1,
		Name:           "Corte",
		Description:    "Corte clasico",
		Duration:       45,
		Price:          150,
		IsActive:       true,
		TreatmentID:    &linked,
	})
	require.NoError(t, err)
	require.NotZero(t, createdID)

	var created models.Service
	require.NoError(t, db.First(&created, createdID).Error)
	require.NotNil(t, created.TreatmentID, "create must persist the treatment link")
	assert.Equal(t, uint(1), *created.TreatmentID)

	// Re-link to a different treatment: a normal update.
	other := uint(2)
	require.NoError(t, svc.Update(created.ID, request.UpdateServiceRequest{TreatmentID: &other}))

	var relinked models.Service
	require.NoError(t, db.First(&relinked, created.ID).Error)
	require.NotNil(t, relinked.TreatmentID)
	assert.Equal(t, uint(2), *relinked.TreatmentID, "a non-zero value sets the link")

	// Unlink: the sentinel 0 must clear it, otherwise a mis-linked service can
	// never be corrected and stays wrongly catalogued forever.
	clear := uint(0)
	require.NoError(t, svc.Update(created.ID, request.UpdateServiceRequest{TreatmentID: &clear}))

	var unlinked models.Service
	require.NoError(t, db.First(&unlinked, created.ID).Error)
	assert.Nil(t, unlinked.TreatmentID, "treatment_id = 0 must clear the link")

	// An update that omits the field entirely must NOT touch the existing link.
	require.NoError(t, svc.Update(created.ID, request.UpdateServiceRequest{TreatmentID: &linked}))
	require.NoError(t, svc.Update(created.ID, request.UpdateServiceRequest{Name: "Corte v2"}))

	var untouched models.Service
	require.NoError(t, db.First(&untouched, created.ID).Error)
	require.NotNil(t, untouched.TreatmentID, "omitting treatment_id must leave the link alone")
	assert.Equal(t, uint(1), *untouched.TreatmentID)
	assert.Equal(t, "Corte v2", untouched.Name)
}
