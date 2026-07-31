package service

import "github.com/Nina-99/yottu-api/models"

type TokenAccountService interface {
	GetOrCreate(beautyCenterID uint, currency string) (*models.TokenAccount, error)
	GetBalance(beautyCenterID uint) (balance, overdraftLimit float64, currency string, err error)
	CanConsume(beautyCenterID uint, amount float64) (bool, error)
	Consume(beautyCenterID uint, amount float64, referenceID uint, description string) error
	ProcessTopUp(topUpID, adminID uint, status string) error
	AddTokens(beautyCenterID uint, amount float64, txType string, referenceID uint, description string) error
}
