package repository

import "github.com/Nina-99/yottu-api/models"

type ClientRepository interface {
	Create(user *models.Client) error
	Update(user *models.Client) error
	Delete(user *models.Client) error
	FindById(userID uint) (*models.Client, error)
	FindAll() ([]models.Client, error)
	FindByEmail(email string) (*models.Client, error)
	FindByUsername(username string) (*models.Client, error)
}
