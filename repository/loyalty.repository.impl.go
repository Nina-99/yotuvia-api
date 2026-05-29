package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type LoyaltyRepositoryImpl struct {
	db *gorm.DB
}

func NewLoyaltyRepositoryImpl(db *gorm.DB) LoyaltyRepository {
	return &LoyaltyRepositoryImpl{db: db}
}

func (r *LoyaltyRepositoryImpl) CreateProgram(program *models.LoyaltyProgram) error {
	return r.db.Create(program).Error
}

func (r *LoyaltyRepositoryImpl) FindProgramByService(beautyCenterID, serviceID uint) (*models.LoyaltyProgram, error) {
	var program models.LoyaltyProgram
	err := r.db.Where("beauty_center_id = ? AND service_id = ? AND is_active", beautyCenterID, serviceID, true).First(&program).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &program, nil
}

func (r *LoyaltyRepositoryImpl) FindActiveCard(clientID, programID uint) (*models.CustomerPunchCard, error) {
	var card models.CustomerPunchCard
	err := r.db.Where("client_id = ? AND loyalty_program_id = ? AND status = ?", clientID, programID, "active").First(&card).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &card, nil
}

func (r *LoyaltyRepositoryImpl) CreatePunchCard(card *models.CustomerPunchCard) error {
	return r.db.Create(card).Error
}

func (r *LoyaltyRepositoryImpl) UpdatePunchCard(card *models.CustomerPunchCard) error {
	return r.db.Save(card).Error
}

func (r *LoyaltyRepositoryImpl) CreatePunchHistory(history *models.PunchHistory) error {
	return r.db.Create(history).Error
}

func (r *LoyaltyRepositoryImpl) FindCardsByClient(clientID uint) ([]models.CustomerPunchCard, error) {
	var cards []models.CustomerPunchCard
	err := r.db.Preload("LoyaltyProgram").Where("client_id = ?", clientID).Find(&cards).Error
	return cards, err
}
