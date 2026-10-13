package repository

import (
	"errors"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type AuthAccountRepositoryImpl struct {
	db *gorm.DB
}

func NewAuthAccountRepositoryImpl(db *gorm.DB) AuthAccountRepository {
	return &AuthAccountRepositoryImpl{db: db}
}

// Create implements AuthAccountRepository
func (r *AuthAccountRepositoryImpl) Create(account *models.AuthAccount) error {
	return r.db.Create(account).Error
}

// Update implements AuthAccountRepository
func (r *AuthAccountRepositoryImpl) Update(account *models.AuthAccount) error {
	return r.db.Save(account).Error
}

// Delete implements AuthAccountRepository
func (r *AuthAccountRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.AuthAccount{}, id).Error
}

// FindAll implements AuthAccountRepository
func (r *AuthAccountRepositoryImpl) FindAll(page, limit int) ([]models.AuthAccount, int64, error) {
	var accounts []models.AuthAccount
	var total int64

	r.db.Model(&models.AuthAccount{}).Count(&total)

	offset := (page - 1) * limit
	err := r.db.Preload("Client").Offset(offset).Limit(limit).Find(&accounts).Error
	return accounts, total, err
}

// FindById implements AuthAccountRepository
func (r *AuthAccountRepositoryImpl) FindById(id uint) (*models.AuthAccount, error) {
	var account models.AuthAccount
	err := r.db.Preload("Client").First(&account, id).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// FindByProvider implements AuthAccountRepository
func (r *AuthAccountRepositoryImpl) FindByProvider(provider, providerId string) (*models.AuthAccount, error) {
	var account models.AuthAccount
	err := r.db.Where("provider = ? AND provider_id = ?", provider, providerId).First(&account).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}

// FindByLocalUser implements AuthAccountRepository. Only one column holds the
// id for a given user type, so the query is branch-free per type.
func (r *AuthAccountRepositoryImpl) FindByLocalUser(userType string, userID uint) (*models.AuthAccount, error) {
	column := ""
	switch userType {
	case "client":
		column = "client_id"
	case "personnel":
		column = "personnel_id"
	case "beauty_center":
		column = "beauty_center_id"
	default:
		return nil, nil
	}

	var account models.AuthAccount
	err := r.db.Where("provider = ? AND user_type = ? AND "+column+" = ?", "local", userType, userID).
		First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}
