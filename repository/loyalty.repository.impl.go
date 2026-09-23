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
	// First try to find a program specific to this service
	var program models.LoyaltyProgram
	err := r.db.Preload("Service").Preload("RewardService").
		Where("beauty_center_id = ? AND service_id = ? AND is_active = ?", beautyCenterID, serviceID, true).
		First(&program).Error
	if err == nil {
		return &program, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	// Fallback: find a program that applies to ALL services (service_id IS NULL)
	err = r.db.Preload("Service").Preload("RewardService").
		Where("beauty_center_id = ? AND service_id IS NULL AND is_active = ?", beautyCenterID, true).
		First(&program).Error
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

func (r *LoyaltyRepositoryImpl) FindCompletedCard(clientID, programID uint) (*models.CustomerPunchCard, error) {
	var card models.CustomerPunchCard
	err := r.db.Where("client_id = ? AND loyalty_program_id = ? AND status = ?", clientID, programID, "completed").First(&card).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &card, nil
}

func (r *LoyaltyRepositoryImpl) FindCompletedCardByCardID(cardID uint) (*models.CustomerPunchCard, error) {
	var card models.CustomerPunchCard
	err := r.db.Preload("LoyaltyProgram").Preload("LoyaltyProgram.Service").Where("id = ? AND status = ?", cardID, "completed").First(&card).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &card, nil
}

func (r *LoyaltyRepositoryImpl) CreateRedemption(redemption *models.LoyaltyRedemption) error {
	return r.db.Create(redemption).Error
}

func (r *LoyaltyRepositoryImpl) FindRedemptionByCard(cardID uint) (*models.LoyaltyRedemption, error) {
	var redemption models.LoyaltyRedemption
	err := r.db.Where("customer_punch_card_id = ?", cardID).First(&redemption).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &redemption, nil
}

func (r *LoyaltyRepositoryImpl) FindRedemptionByReservation(reservationID uint) (*models.LoyaltyRedemption, error) {
	var redemption models.LoyaltyRedemption
	err := r.db.Where("reservation_id = ?", reservationID).First(&redemption).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &redemption, nil
}

func (r *LoyaltyRepositoryImpl) FindProgramsByCenter(beautyCenterID uint) ([]models.LoyaltyProgram, error) {
	var programs []models.LoyaltyProgram
	err := r.db.Preload("Service").Preload("RewardService").Where("beauty_center_id = ?", beautyCenterID).Order("created_at DESC").Find(&programs).Error
	return programs, err
}

func (r *LoyaltyRepositoryImpl) FindProgramByID(id uint) (*models.LoyaltyProgram, error) {
	var program models.LoyaltyProgram
	err := r.db.Preload("Service").Preload("RewardService").First(&program, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &program, nil
}

func (r *LoyaltyRepositoryImpl) UpdateProgram(program *models.LoyaltyProgram) error {
	return r.db.Table("loyalty_programs").Where("id = ?", program.ID).UpdateColumns(map[string]interface{}{
		"service_id":        program.ServiceID,
		"reward_service_id": program.RewardServiceID,
		"required_steps":    program.RequiredSteps,
		"reward_type":       program.RewardType,
		"discount_value":    program.DiscountValue,
		"is_active":         program.IsActive,
	}).Error
}

func (r *LoyaltyRepositoryImpl) DeleteProgram(program *models.LoyaltyProgram) error {
	return r.db.Delete(program).Error
}

func (r *LoyaltyRepositoryImpl) FindActiveCardForPersonnel(clientID, programID, personnelID uint) (*models.CustomerPunchCard, error) {
	var card models.CustomerPunchCard
	err := r.db.Where("client_id = ? AND loyalty_program_id = ? AND status = ? AND personnel_id = ?", clientID, programID, "active", personnelID).First(&card).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &card, nil
}

func (r *LoyaltyRepositoryImpl) UpdateProgramsActiveByCenter(beautyCenterID uint, isActive bool) error {
	return r.db.Table("loyalty_programs").
		Where("beauty_center_id = ?", beautyCenterID).
		UpdateColumns(map[string]interface{}{"is_active": isActive}).
		Error
}
