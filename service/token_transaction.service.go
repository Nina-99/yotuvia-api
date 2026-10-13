package service

import "github.com/Nina-99/yotuvia-api/models"

type TokenTransactionService interface {
	GetHistory(beautyCenterID uint, page, limit int) ([]models.TokenTransaction, int64, error)
	// GetSummary returns totals in milli-YOTTUs. Historical "grace" rows are
	// read-only and excluded from total_consumed.
	GetSummary(beautyCenterID uint) (totalConsumed, totalTopUps int64, err error)
}
