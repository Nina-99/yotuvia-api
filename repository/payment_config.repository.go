package repository

import "github.com/Nina-99/yottu-api/models"

//go:generate mockery --name=PaymentConfigRepository --output=../mocks --outpkg=mocks
type PaymentConfigRepository interface {
	Get() (*models.PaymentConfig, error)
	Save(config *models.PaymentConfig) error
}
