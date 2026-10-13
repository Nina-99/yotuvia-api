package service

import (
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
)

var personnelAllowedImageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
}

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
		IsProfile:   req.IsProfile,
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
		IsProfile:   personnelImage.IsProfile,
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
		IsProfile:   personnelImage.IsProfile,
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
			IsProfile:   img.IsProfile,
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

func (s *PersonnelImageServiceImpl) Upload(personnelID uint, file *multipart.FileHeader, isCover bool, isProfile bool) (response.PersonnelImageResponse, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !personnelAllowedImageExtensions[ext] {
		return response.PersonnelImageResponse{}, fmt.Errorf("invalid image extension: %s (allowed: jpg, jpeg, png, webp, gif)", ext)
	}

	if file.Size > 10*1024*1024 {
		return response.PersonnelImageResponse{}, fmt.Errorf("file too large: max 10MB")
	}

	// Verify if personnel exists
	_, err := s.PersonnelRepository.FindByID(personnelID)
	if err != nil {
		return response.PersonnelImageResponse{}, errors.New("personnel not found")
	}

	timestamp := time.Now().UnixNano()
	safeName := fmt.Sprintf("%d_%d%s", personnelID, timestamp, ext)
	uploadDir := "uploads/personnel"
	filePath := filepath.Join(uploadDir, safeName)

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return response.PersonnelImageResponse{}, fmt.Errorf("failed to create upload dir: %w", err)
	}

	if err := utils.SaveUploadedFile(file, filePath); err != nil {
		return response.PersonnelImageResponse{}, fmt.Errorf("failed to save file: %w", err)
	}

	imageURL := fmt.Sprintf("/uploads/personnel/%s", safeName)

	personnelImage := models.PersonnelImage{
		PersonnelID: personnelID,
		ImageUrl:    imageURL,
		IsCover:     isCover,
		IsProfile:   isProfile,
		Position:    0,
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
		IsProfile:   personnelImage.IsProfile,
		Position:    personnelImage.Position,
		CreatedAt:   personnelImage.CreatedAt,
	}, nil
}
