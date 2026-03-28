package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type ClientRepositoryImpl struct {
	db *gorm.DB
}

func NewClientRepositoryImpl(db *gorm.DB) ClientRepository {
	return &ClientRepositoryImpl{db: db}
}

// Create implements ClientRepository
func (r *ClientRepositoryImpl) Create(client *models.Client) error {
	return r.db.Create(&client).Error
}

// Update implements ClientRepository
func (r *ClientRepositoryImpl) Update(client *models.Client) error {
	return r.db.Save(client).Error
}

// Delete implements ClientRepository
func (r *ClientRepositoryImpl) Delete(client *models.Client) error {
	return r.db.Delete(client).Error
}

// FindAll implements ClientRepository
func (r *ClientRepositoryImpl) FindAll() ([]models.Client, error) {
	var users []models.Client
	err := r.db.Find(&users).Error
	return users, err
}

// FindById implements ClientRepository
func (r *ClientRepositoryImpl) FindById(clienId uint) (*models.Client, error) {
	var user models.Client
	err := r.db.First(&user, clienId).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByEmail implements ClientRepository
func (r *ClientRepositoryImpl) FindByEmail(email string) (*models.Client, error) {
	var client models.Client
	err := r.db.Where("email = ?", email).First(&client).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &client, nil
}

// FindByUsername implements ClientRepository
func (r *ClientRepositoryImpl) FindByUsername(username string) (*models.Client, error) {
	var client models.Client
	err := r.db.Where("username = ?", username).First(&client).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &client, nil
}
