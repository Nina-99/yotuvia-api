package service

import (
	"encoding/json"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
)

type PaymentConfigServiceImpl struct {
	repo repository.PaymentConfigRepository
}

func NewPaymentConfigServiceImpl(repo repository.PaymentConfigRepository) *PaymentConfigServiceImpl {
	return &PaymentConfigServiceImpl{repo: repo}
}

func (s *PaymentConfigServiceImpl) Get(centerID uint) (*models.PaymentConfig, error) {
	return s.repo.GetByCenter(centerID)
}

func (s *PaymentConfigServiceImpl) GetTiers(centerID uint) (models.PaymentConfigTiers, error) {
	config, err := s.repo.GetByCenter(centerID)
	if err != nil {
		return nil, err
	}
	return s.parseTiers(config.TiersJSON)
}

func (s *PaymentConfigServiceImpl) Update(centerID uint, qrURL, phone string, tiers models.PaymentConfigTiers) error {
	config, err := s.repo.GetByCenter(centerID)
	if err != nil {
		return err
	}

	// Re-assert ownership from the argument, never from the loaded row: the
	// config a handler passes in came from the caller's token claim.
	config.BeautyCenterID = centerID

	if qrURL != "" {
		config.PaymentQRURL = qrURL
	}
	if phone != "" {
		config.PaymentPhone = phone
	}

	if tiers != nil {
		data, err := json.Marshal(tiers)
		if err != nil {
			return err
		}
		config.TiersJSON = string(data)
	}

	return s.repo.Save(config)
}

func (s *PaymentConfigServiceImpl) parseTiers(raw string) (models.PaymentConfigTiers, error) {
	if raw == "" {
		return models.PaymentConfigTiers{}, nil
	}
	var tiers models.PaymentConfigTiers
	if err := json.Unmarshal([]byte(raw), &tiers); err != nil {
		return nil, err
	}
	return tiers, nil
}
