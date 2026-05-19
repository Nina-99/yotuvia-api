package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/go-playground/validator/v10"
)

type BeautyCenterImageServiceImpl struct {
	BeautyCenterImageRepository repository.BeautyCenterImageRepository
	BeautyCenterRepository      repository.BeautyCenterRepository
	Validate                    *validator.Validate
}

func NewBeautyCenterImageServiceImpl(beautyCenterImageRepository repository.BeautyCenterImageRepository, beautyCenterRepository repository.BeautyCenterRepository, validate *validator.Validate) BeautyCenterImageService {
	return &BeautyCenterImageServiceImpl{
		BeautyCenterImageRepository: beautyCenterImageRepository,
		BeautyCenterRepository:      beautyCenterRepository,
		Validate:                    validate,
	}
}

func (s *BeautyCenterImageServiceImpl) Create(req request.BeautyCenterImageRequest) (response.BeautyCenterImageResp, error) {
	err := s.Validate.Struct(req)
	if err != nil {
		return response.BeautyCenterImageResp{}, err
	}

	// Verify if personnel exists
	_, err = s.BeautyCenterRepository.FindByID(req.BeautyCenterID)
	if err != nil {
		return response.BeautyCenterImageResp{}, errors.New("personnel not found")
	}

	personnelImage := models.BeautyCenterImage{
		BeautyCenterID: req.BeautyCenterID,
		ImageUrl:       req.ImageUrl,
		IsCover:        req.IsCover,
		Position:       req.Position,
	}

	err = s.BeautyCenterImageRepository.Save(&personnelImage)
	if err != nil {
		return response.BeautyCenterImageResp{}, err
	}

	return response.BeautyCenterImageResp{
		ID:             personnelImage.ID,
		BeautyCenterID: personnelImage.BeautyCenterID,
		ImageUrl:       personnelImage.ImageUrl,
		IsCover:        personnelImage.IsCover,
		Position:       personnelImage.Position,
		CreatedAt:      personnelImage.CreatedAt,
	}, nil
}

func (s *BeautyCenterImageServiceImpl) GetByID(id uint) (response.BeautyCenterImageResp, error) {
	personnelImage, err := s.BeautyCenterImageRepository.FindByID(id)
	if err != nil {
		return response.BeautyCenterImageResp{}, err
	}

	return response.BeautyCenterImageResp{
		ID:             personnelImage.ID,
		BeautyCenterID: personnelImage.BeautyCenterID,
		ImageUrl:       personnelImage.ImageUrl,
		IsCover:        personnelImage.IsCover,
		Position:       personnelImage.Position,
		CreatedAt:      personnelImage.CreatedAt,
	}, nil
}

func (s *BeautyCenterImageServiceImpl) GetByBeautyCenterID(beauterCenterID uint) ([]response.BeautyCenterImageResp, error) {
	// Verify if personnel exists
	_, err := s.BeautyCenterRepository.FindByID(beauterCenterID)
	if err != nil {
		return nil, errors.New("personnel not found")
	}

	personnelImages, err := s.BeautyCenterImageRepository.FindByBeautyCenterID(beauterCenterID)
	if err != nil {
		return nil, err
	}

	var res []response.BeautyCenterImageResp
	for _, img := range personnelImages {
		res = append(res, response.BeautyCenterImageResp{
			ID:             img.ID,
			BeautyCenterID: img.BeautyCenterID,
			ImageUrl:       img.ImageUrl,
			IsCover:        img.IsCover,
			Position:       img.Position,
			CreatedAt:      img.CreatedAt,
		})
	}
	if res == nil {
		res = []response.BeautyCenterImageResp{}
	}

	return res, nil
}

func (s *BeautyCenterImageServiceImpl) Delete(id uint) error {
	_, err := s.BeautyCenterImageRepository.FindByID(id)
	if err != nil {
		return err
	}

	return s.BeautyCenterImageRepository.Delete(id)
}
