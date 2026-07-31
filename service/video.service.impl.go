package service

import (
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
)

type VideoServiceImpl struct {
	repo repository.VideoRepository
}

func NewVideoServiceImpl(repo repository.VideoRepository) VideoService {
	return &VideoServiceImpl{repo: repo}
}

var allowedExtensions = map[string]bool{
	".mp4":  true,
	".mov":  true,
	".avi":  true,
	".mkv":  true,
	".webm": true,
}

func (s *VideoServiceImpl) Upload(centerID uint, title, description string, file *multipart.FileHeader) (*models.Video, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedExtensions[ext] {
		return nil, fmt.Errorf("invalid file extension: %s", ext)
	}

	if file.Size > 100*1024*1024 {
		return nil, fmt.Errorf("file too large: max 100MB")
	}

	// Generate unique filename
	timestamp := time.Now().UnixNano()
	safeName := fmt.Sprintf("%d_%d%s", centerID, timestamp, ext)
	uploadDir := "uploads/videos"
	filePath := filepath.Join(uploadDir, safeName)

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload dir: %w", err)
	}

	if err := utils.SaveUploadedFile(file, filePath); err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	video := &models.Video{
		BeautyCenterID: centerID,
		Title:          title,
		Description:    description,
		FileName:       safeName,
		FilePath:       filePath,
		IsActive:       true,
	}

	if err := s.repo.Create(video); err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save video record: %w", err)
	}

	utils.Log.Infof("[VIDEO] Uploaded: %s by center %d (size: %d)", safeName, centerID, file.Size)
	return video, nil
}

func (s *VideoServiceImpl) Delete(videoID, centerID uint) error {
	video, err := s.repo.FindByID(videoID)
	if err != nil {
		return err
	}
	if video.BeautyCenterID != centerID {
		return fmt.Errorf("unauthorized: this video does not belong to your center")
	}

	os.Remove(video.FilePath)
	return s.repo.Delete(video)
}

func (s *VideoServiceImpl) GetFeed(page, limit int) ([]models.Video, int64, error) {
	return s.repo.FindAll(page, limit)
}

func (s *VideoServiceImpl) GetByCenter(centerID uint, page, limit int) ([]models.Video, int64, error) {
	return s.repo.FindByBeautyCenterID(centerID, page, limit)
}

func (s *VideoServiceImpl) IncrementViews(videoID uint) error {
	return s.repo.IncrementViews(videoID)
}
