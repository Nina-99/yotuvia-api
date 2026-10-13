package repository

import "github.com/Nina-99/yotuvia-api/models"

type TokenTransactionRepository interface {
	Create(tx *models.TokenTransaction) error
	FindByBeautyCenterID(beautyCenterID uint, page, limit int) ([]models.TokenTransaction, int64, error)
}
