package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
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

func (s *CategoryServiceImpl) Create(req request.CreateCategoryRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	category := models.Category{
		BeautyCenterID: req.BeautyCenterID,
		Name:           req.Name,
		Description:    req.Description,
		Icon:        req.Icon,
	}

	return s.CategoryRepository.Create(&category)
}

func (s *CategoryServiceImpl) Update(categoryID uint, req request.CreateCategoryRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	category, err := s.CategoryRepository.FindById(categoryID)
	if err != nil || category == nil {
		return errors.New("category not found")
	}

	category.BeautyCenterID = req.BeautyCenterID
	category.Name = req.Name
	category.Description = req.Description
	category.Icon = req.Icon

	return s.CategoryRepository.Update(category)
}

func (s *CategoryServiceImpl) Delete(categoryID uint) error {
	category, err := s.CategoryRepository.FindById(categoryID)
	if err != nil || category == nil {
		return errors.New("category not found")
	}

	return s.CategoryRepository.Delete(category)
}

func (s *CategoryServiceImpl) FindAll(beautyCenterID uint, page, limit int) ([]models.Category, int64, error) {
	return s.CategoryRepository.FindAll(beautyCenterID, page, limit)
}

func (s *CategoryServiceImpl) FindById(categoryID uint) (*models.Category, error) {
	return s.CategoryRepository.FindById(categoryID)
}
