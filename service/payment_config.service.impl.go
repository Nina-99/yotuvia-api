package service

import (
	"encoding/json"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
)

type PaymentConfigServiceImpl struct {
	repo repository.PaymentConfigRepository
}

func NewPaymentConfigServiceImpl(repo repository.PaymentConfigRepository) *PaymentConfigServiceImpl {
	return &PaymentConfigServiceImpl{repo: repo}
}

func (s *PaymentConfigServiceImpl) Get() (*models.PaymentConfig, error) {
	return s.repo.Get()
}

func (s *PaymentConfigServiceImpl) GetTiers() (models.PaymentConfigTiers, error) {
	config, err := s.repo.Get()
	if err != nil {
		return nil, err
	}
	return s.parseTiers(config.TiersJSON)
}

func (s *PaymentConfigServiceImpl) Update(qrURL, phone string, tiers models.PaymentConfigTiers) error {
	config, err := s.repo.Get()
	if err != nil {
		return err
	}

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
