package service

import "github.com/Nina-99/yottu-api/models"

type TokenTransactionService interface {
	GetHistory(beautyCenterID uint, page, limit int) ([]models.TokenTransaction, int64, error)
	GetSummary(beautyCenterID uint) (totalConsumed, totalTopUps float64, err error)
}
