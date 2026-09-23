package service

import "github.com/Nina-99/yottu-api/models"

type PaymentConfigService interface {
	Get() (*models.PaymentConfig, error)
	GetTiers() (models.PaymentConfigTiers, error)
	Update(qrURL, phone string, tiers models.PaymentConfigTiers) error
}
