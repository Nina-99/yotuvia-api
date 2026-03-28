package service

import (
	"errors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/go-playground/validator/v10"
)

type PersonnelImageServiceImpl struct {
	PersonnelImageRepository repository.PersonnelImageRepository
	PersonnelRepository      repository.PersonnelRepository
	Validate                 *validator.Validate
}

func NewPersonnelImageServiceImpl(personnelImageRepository repository.PersonnelImageRepository, personnelRepository repository.PersonnelRepository, validate *validator.Validate) PersonnelImageService {
	return &PersonnelImageServiceImpl{
		PersonnelImageRepository: personnelImageRepository,
		PersonnelRepository:      personnelRepository,
		Validate:                 validate,
	}
}

func (s *PersonnelImageServiceImpl) Create(req request.CreatePersonnelImageRequest) (response.PersonnelImageResponse, error) {
	err := s.Validate.Struct(req)
	if err != nil {
		return response.PersonnelImageResponse{}, err
	}

	// Verify if personnel exists
	_, err = s.PersonnelRepository.FindByID(req.PersonnelID)
	if err != nil {
		return response.PersonnelImageResponse{}, errors.New("personnel not found")
	}

	personnelImage := models.PersonnelImage{
		PersonnelID: req.PersonnelID,
		ImageUrl:    req.ImageUrl,
		IsCover:     req.IsCover,
		Position:    req.Position,
	}

	err = s.PersonnelImageRepository.Save(&personnelImage)
	if err != nil {
		return response.PersonnelImageResponse{}, err
	}

	return response.PersonnelImageResponse{
		ID:          personnelImage.ID,
		PersonnelID: personnelImage.PersonnelID,
		ImageUrl:    personnelImage.ImageUrl,
		IsCover:     personnelImage.IsCover,
		Position:    personnelImage.Position,
		CreatedAt:   personnelImage.CreatedAt,
	}, nil
}

func (s *PersonnelImageServiceImpl) GetByID(id uint) (response.PersonnelImageResponse, error) {
	personnelImage, err := s.PersonnelImageRepository.FindByID(id)
	if err != nil {
		return response.PersonnelImageResponse{}, err
	}

	return response.PersonnelImageResponse{
		ID:          personnelImage.ID,
		PersonnelID: personnelImage.PersonnelID,
		ImageUrl:    personnelImage.ImageUrl,
		IsCover:     personnelImage.IsCover,
		Position:    personnelImage.Position,
		CreatedAt:   personnelImage.CreatedAt,
	}, nil
}

func (s *PersonnelImageServiceImpl) GetByPersonnelID(personnelID uint) ([]response.PersonnelImageResponse, error) {
	// Verify if personnel exists
	_, err := s.PersonnelRepository.FindByID(personnelID)
	if err != nil {
		return nil, errors.New("personnel not found")
	}

	personnelImages, err := s.PersonnelImageRepository.FindByPersonnelID(personnelID)
	if err != nil {
		return nil, err
	}

	var res []response.PersonnelImageResponse
	for _, img := range personnelImages {
		res = append(res, response.PersonnelImageResponse{
			ID:          img.ID,
			PersonnelID: img.PersonnelID,
			ImageUrl:    img.ImageUrl,
			IsCover:     img.IsCover,
			Position:    img.Position,
			CreatedAt:   img.CreatedAt,
		})
	}
	if res == nil {
		res = []response.PersonnelImageResponse{}
	}

	return res, nil
}

func (s *PersonnelImageServiceImpl) Delete(id uint) error {
	_, err := s.PersonnelImageRepository.FindByID(id)
	if err != nil {
		return err
	}

	return s.PersonnelImageRepository.Delete(id)
}
