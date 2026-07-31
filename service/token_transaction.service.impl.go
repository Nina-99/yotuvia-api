package service

import (
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
)

type TokenTransactionServiceImpl struct {
	txRepo repository.TokenTransactionRepository
}

func NewTokenTransactionServiceImpl(txRepo repository.TokenTransactionRepository) TokenTransactionService {
	return &TokenTransactionServiceImpl{txRepo: txRepo}
}

func (s *TokenTransactionServiceImpl) GetHistory(beautyCenterID uint, page, limit int) ([]models.TokenTransaction, int64, error) {
	return s.txRepo.FindByBeautyCenterID(beautyCenterID, page, limit)
}

func (s *TokenTransactionServiceImpl) GetSummary(beautyCenterID uint) (totalConsumed, totalTopUps float64, err error) {
	txs, _, err := s.txRepo.FindByBeautyCenterID(beautyCenterID, 1, 10000)
	if err != nil {
		return 0, 0, err
	}

	for _, tx := range txs {
		if tx.Type == "consumption" || tx.Type == "grace" {
			totalConsumed += -tx.Amount
		} else if tx.Type == "topup" {
			totalTopUps += tx.Amount
		}
	}

	return totalConsumed, totalTopUps, nil
}
