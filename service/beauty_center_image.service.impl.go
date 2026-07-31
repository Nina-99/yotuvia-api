package service

import (
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
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

var allowedImageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
}

func mapImageToResp(img models.BeautyCenterImage) response.BeautyCenterImageResp {
	return response.BeautyCenterImageResp{
		ID:             img.ID,
		BeautyCenterID: img.BeautyCenterID,
		ImageUrl:       img.ImageUrl,
		IsCover:        img.IsCover,
		IsProfile:      img.IsProfile,
		Position:       img.Position,
		CreatedAt:      img.CreatedAt,
	}
}

func (s *BeautyCenterImageServiceImpl) Upload(beautyCenterID uint, file *multipart.FileHeader, isCover bool, isProfile bool) (response.BeautyCenterImageResp, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExtensions[ext] {
		return response.BeautyCenterImageResp{}, fmt.Errorf("invalid image extension: %s (allowed: jpg, jpeg, png, webp, gif)", ext)
	}

	if file.Size > 10*1024*1024 {
		return response.BeautyCenterImageResp{}, fmt.Errorf("file too large: max 10MB")
	}

	timestamp := time.Now().UnixNano()
	safeName := fmt.Sprintf("%d_%d%s", beautyCenterID, timestamp, ext)
	uploadDir := "uploads/images"
	filePath := filepath.Join(uploadDir, safeName)

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return response.BeautyCenterImageResp{}, fmt.Errorf("failed to create upload dir: %w", err)
	}

	if err := utils.SaveUploadedFile(file, filePath); err != nil {
		return response.BeautyCenterImageResp{}, fmt.Errorf("failed to save file: %w", err)
	}

	imageURL := fmt.Sprintf("/uploads/images/%s", safeName)

	img := models.BeautyCenterImage{
		BeautyCenterID: beautyCenterID,
		ImageUrl:       imageURL,
		IsCover:        isCover,
		IsProfile:      isProfile,
	}

	if err := s.BeautyCenterImageRepository.Save(&img); err != nil {
		return response.BeautyCenterImageResp{}, fmt.Errorf("failed to save image record: %w", err)
	}

	return mapImageToResp(img), nil
}

func (s *BeautyCenterImageServiceImpl) Create(req request.BeautyCenterImageRequest) (response.BeautyCenterImageResp, error) {
	err := s.Validate.Struct(req)
	if err != nil {
		return response.BeautyCenterImageResp{}, err
	}

	_, err = s.BeautyCenterRepository.FindByID(req.BeautyCenterID)
	if err != nil {
		return response.BeautyCenterImageResp{}, errors.New("beauty center not found")
	}

	personnelImage := models.BeautyCenterImage{
		BeautyCenterID: req.BeautyCenterID,
		ImageUrl:       req.ImageUrl,
		IsCover:        req.IsCover,
		IsProfile:      req.IsProfile,
		Position:       req.Position,
	}

	err = s.BeautyCenterImageRepository.Save(&personnelImage)
	if err != nil {
		return response.BeautyCenterImageResp{}, err
	}

	return mapImageToResp(personnelImage), nil
}

func (s *BeautyCenterImageServiceImpl) Update(id uint, req request.UpdateBeautyCenterImageRequest) (response.BeautyCenterImageResp, error) {
	img, err := s.BeautyCenterImageRepository.FindByID(id)
	if err != nil {
		return response.BeautyCenterImageResp{}, err
	}

	if req.IsCover != nil {
		img.IsCover = *req.IsCover
	}
	if req.IsProfile != nil {
		img.IsProfile = *req.IsProfile
	}
	if req.Position != nil {
		img.Position = *req.Position
	}

	if err := s.BeautyCenterImageRepository.Save(img); err != nil {
		return response.BeautyCenterImageResp{}, err
	}

	return mapImageToResp(*img), nil
}

func (s *BeautyCenterImageServiceImpl) Reorder(centerID uint, req request.ReorderImagesRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	for _, item := range req.Order {
		img, err := s.BeautyCenterImageRepository.FindByID(item.ID)
		if err != nil {
			return fmt.Errorf("image %d not found", item.ID)
		}
		if img.BeautyCenterID != centerID {
			return fmt.Errorf("image %d does not belong to this center", item.ID)
		}
		img.Position = item.Position
		if err := s.BeautyCenterImageRepository.Save(img); err != nil {
			return err
		}
	}
	return nil
}

func (s *BeautyCenterImageServiceImpl) GetByID(id uint) (response.BeautyCenterImageResp, error) {
	img, err := s.BeautyCenterImageRepository.FindByID(id)
	if err != nil {
		return response.BeautyCenterImageResp{}, err
	}
	return mapImageToResp(*img), nil
}

func (s *BeautyCenterImageServiceImpl) GetByBeautyCenterID(beauterCenterID uint) ([]response.BeautyCenterImageResp, error) {
	_, err := s.BeautyCenterRepository.FindByID(beauterCenterID)
	if err != nil {
		return nil, errors.New("beauty center not found")
	}

	images, err := s.BeautyCenterImageRepository.FindByBeautyCenterID(beauterCenterID)
	if err != nil {
		return nil, err
	}

	res := make([]response.BeautyCenterImageResp, 0, len(images))
	for _, img := range images {
		res = append(res, mapImageToResp(img))
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
