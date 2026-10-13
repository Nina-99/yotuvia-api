package repository

import (
	"github.com/Nina-99/yotuvia-api/models"
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
	err := r.db.Preload("LoyaltyProgram").Preload("LoyaltyProgram.Service").Where("client_id = ?", clientID).Find(&cards).Error
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
		"duration_days":     program.DurationDays,
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

// FindPunchHistoryByClient returns punch rows for a client's cards, newest
// first. Cards are resolved first so the history is scoped to the client even
// though punch_histories carries no client column.
func (r *LoyaltyRepositoryImpl) FindPunchHistoryByClient(clientID uint) ([]models.PunchHistory, error) {
	var cards []models.CustomerPunchCard
	if err := r.db.Where("client_id = ?", clientID).Find(&cards).Error; err != nil {
		return nil, err
	}
	if len(cards) == 0 {
		return []models.PunchHistory{}, nil
	}
	cardIDs := make([]uint, 0, len(cards))
	for _, c := range cards {
		cardIDs = append(cardIDs, c.ID)
	}
	var rows []models.PunchHistory
	err := r.db.Preload("CustomerPunchCard.LoyaltyProgram.Service").
		Where("customer_punch_card_id IN ?", cardIDs).
		Order("created_at DESC").Find(&rows).Error
	return rows, err
}

// FindCompletedCardsByClient returns completed cards with program preloaded.
func (r *LoyaltyRepositoryImpl) FindCompletedCardsByClient(clientID uint) ([]models.CustomerPunchCard, error) {
	var cards []models.CustomerPunchCard
	err := r.db.Preload("LoyaltyProgram").Preload("LoyaltyProgram.Service").
		Where("client_id = ? AND status = ?", clientID, "completed").Find(&cards).Error
	return cards, err
}

// FindPublicProgramsByCenter returns active programs for anonymous display.
func (r *LoyaltyRepositoryImpl) FindPublicProgramsByCenter(beautyCenterID uint) ([]models.LoyaltyProgram, error) {
	var programs []models.LoyaltyProgram
	err := r.db.Preload("Service").Preload("RewardService").
		Where("beauty_center_id = ? AND is_active = ?", beautyCenterID, true).
		Order("created_at DESC").Find(&programs).Error
	return programs, err
}

// FindPunchHistoryByReservationCard backs the UNIQUE double-punch guard:
// a concurrent re-scan finds the existing row and treats it as no-op success.
func (r *LoyaltyRepositoryImpl) FindPunchHistoryByReservationCard(reservationID, cardID uint) (*models.PunchHistory, error) {
	var row models.PunchHistory
	err := r.db.Where("reservation_id = ? AND customer_punch_card_id = ?", reservationID, cardID).First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// CountCardsByProgramStatus returns (active, completed) card counts for a
// program. Soft-deleted rows are excluded by GORM's default scope.
func (r *LoyaltyRepositoryImpl) CountCardsByProgramStatus(programID uint) (int64, int64, error) {
	var active int64
	if err := r.db.Model(&models.CustomerPunchCard{}).
		Where("loyalty_program_id = ? AND status = ?", programID, "active").
		Count(&active).Error; err != nil {
		return 0, 0, err
	}
	var completed int64
	if err := r.db.Model(&models.CustomerPunchCard{}).
		Where("loyalty_program_id = ? AND status = ?", programID, "completed").
		Count(&completed).Error; err != nil {
		return 0, 0, err
	}
	return active, completed, nil
}

// CountRedemptionsByProgram counts redemptions whose card belongs to the
// program (redemptions carry no program column, so join through the card).
func (r *LoyaltyRepositoryImpl) CountRedemptionsByProgram(programID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.LoyaltyRedemption{}).
		Joins("JOIN customer_punch_cards ON customer_punch_cards.id = loyalty_redemptions.customer_punch_card_id").
		Where("customer_punch_cards.loyalty_program_id = ?", programID).
		Count(&count).Error
	return count, err
}
