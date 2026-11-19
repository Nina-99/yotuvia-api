package service

import (
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
)

type CategoryService interface {
	// Create/Update/Delete are centre-scoped: actorCenterID is the caller's
	// `center_id` claim. A foreign id is ErrForbidden and never reaches a write.
	Create(actorCenterID uint, req request.CreateCategoryRequest) error
	Update(actorCenterID uint, categoryID uint, req request.CreateCategoryRequest) error
	Delete(actorCenterID uint, categoryID uint) error
	FindAll(beautyCenterID uint, page, limit int) ([]models.Category, int64, error)
	FindById(categoryID uint) (*models.Category, error)
}
