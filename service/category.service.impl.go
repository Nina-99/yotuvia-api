package service

import (
	"fmt"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
)

type CategoryServiceImpl struct {
	CategoryRepository repository.CategoryRepository
	Validate           *validator.Validate
}

func NewCategoryServiceImpl(categoryRepository repository.CategoryRepository, validate *validator.Validate) CategoryService {
	return &CategoryServiceImpl{
		CategoryRepository: categoryRepository,
		Validate:           validate,
	}
}

// Create adds a category to the CALLER'S centre. The owner comes from the JWT
// `center_id` claim and overwrites the body: a category is centre-owned data,
// and the previous version let any authenticated account create one anywhere.
func (s *CategoryServiceImpl) Create(actorCenterID uint, req request.CreateCategoryRequest) error {
	if actorCenterID == 0 {
		return apperrors.ErrForbidden
	}
	req.BeautyCenterID = actorCenterID

	if err := s.Validate.Struct(req); err != nil {
		return fmt.Errorf("%w: %v", apperrors.ErrValidation, err)
	}

	category := models.Category{
		BeautyCenterID:  actorCenterID,
		Name:            req.Name,
		Description:     req.Description,
		NameI18n:        utils.NormalizeI18n(req.NameI18n),
		DescriptionI18n: utils.NormalizeI18n(req.DescriptionI18n),
		Icon:            req.Icon,
	}

	return s.CategoryRepository.Create(&category)
}

// Update edits a category owned by the caller's centre. Ownership is checked
// before any write and the category can never move to another centre
// (`beauty_center_id` in the body is ignored).
func (s *CategoryServiceImpl) Update(actorCenterID uint, categoryID uint, req request.CreateCategoryRequest) error {
	if actorCenterID == 0 {
		return apperrors.ErrForbidden
	}
	if err := s.Validate.Struct(req); err != nil {
		return fmt.Errorf("%w: %v", apperrors.ErrValidation, err)
	}

	category, err := s.CategoryRepository.FindById(categoryID)
	if err != nil || category == nil {
		return apperrors.ErrNotFound
	}
	if category.BeautyCenterID != actorCenterID {
		return apperrors.ErrForbidden
	}

	category.Name = req.Name
	category.Description = req.Description
	category.NameI18n = utils.NormalizeI18n(req.NameI18n)
	category.DescriptionI18n = utils.NormalizeI18n(req.DescriptionI18n)
	category.Icon = req.Icon

	return s.CategoryRepository.Update(category)
}

// Delete removes a category owned by the caller's centre.
func (s *CategoryServiceImpl) Delete(actorCenterID uint, categoryID uint) error {
	if actorCenterID == 0 {
		return apperrors.ErrForbidden
	}
	category, err := s.CategoryRepository.FindById(categoryID)
	if err != nil || category == nil {
		return apperrors.ErrNotFound
	}
	if category.BeautyCenterID != actorCenterID {
		return apperrors.ErrForbidden
	}

	return s.CategoryRepository.Delete(category)
}

func (s *CategoryServiceImpl) FindAll(beautyCenterID uint, page, limit int) ([]models.Category, int64, error) {
	return s.CategoryRepository.FindAll(beautyCenterID, page, limit)
}

func (s *CategoryServiceImpl) FindById(categoryID uint) (*models.Category, error) {
	return s.CategoryRepository.FindById(categoryID)
}
