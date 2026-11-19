package configs

import (
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestEnsureTopUpLedgerUniqueIndex proves the partial index is created and that
// it blocks a SECOND "topup" credit for the same reference_id while leaving
// other transaction types untouched.
func TestEnsureTopUpLedgerUniqueIndex(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.TokenTransaction{}))

	ensureTopUpLedgerUniqueIndex(db)

	ref := uint(7)
	assert.NoError(t, db.Create(&models.TokenTransaction{
		BeautyCenterID: 1, Amount: 12500, Type: "topup", ReferenceID: &ref,
	}).Error)
	err = db.Create(&models.TokenTransaction{
		BeautyCenterID: 1, Amount: 12500, Type: "topup", ReferenceID: &ref,
	}).Error
	assert.Error(t, err, "a second topup credit for the same reference must be rejected")

	// A different type sharing the reference id is NOT constrained.
	assert.NoError(t, db.Create(&models.TokenTransaction{
		BeautyCenterID: 1, Amount: 100, Type: "bonus", ReferenceID: &ref,
	}).Error)
}
