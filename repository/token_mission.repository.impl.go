package repository

import (
	"github.com/Nina-99/yottu-api/models"
	"gorm.io/gorm"
)

type TokenMissionRepositoryImpl struct {
	db *gorm.DB
}

func NewTokenMissionRepositoryImpl(db *gorm.DB) TokenMissionRepository {
	return &TokenMissionRepositoryImpl{db: db}
}

func (r *TokenMissionRepositoryImpl) CreateMission(mission *models.TokenMission) error {
	return r.db.Create(mission).Error
}

func (r *TokenMissionRepositoryImpl) UpdateMission(mission *models.TokenMission) error {
	return r.db.Save(mission).Error
}

func (r *TokenMissionRepositoryImpl) FindMissionByID(id uint) (*models.TokenMission, error) {
	var mission models.TokenMission
	err := r.db.First(&mission, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &mission, nil
}

func (r *TokenMissionRepositoryImpl) FindActiveMissions() ([]models.TokenMission, error) {
	var missions []models.TokenMission
	err := r.db.Where("is_active = ?", true).Order("sort_order ASC").Find(&missions).Error
	return missions, err
}

func (r *TokenMissionRepositoryImpl) FindMissionByType(triggerType string) (*models.TokenMission, error) {
	var mission models.TokenMission
	err := r.db.Where("trigger_type = ? AND is_active = ?", triggerType, true).First(&mission).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &mission, nil
}

func (r *TokenMissionRepositoryImpl) UpsertProgress(progress *models.TokenMissionProgress) error {
	var existing models.TokenMissionProgress
	err := r.db.Where("beauty_center_id = ? AND mission_id = ?", progress.BeautyCenterID, progress.MissionID).First(&existing).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return r.db.Create(progress).Error
		}
		return err
	}
	progress.ID = existing.ID
	progress.CreatedAt = existing.CreatedAt
	return r.db.Save(progress).Error
}

func (r *TokenMissionRepositoryImpl) FindProgress(beautyCenterID, missionID uint) (*models.TokenMissionProgress, error) {
	var progress models.TokenMissionProgress
	err := r.db.Where("beauty_center_id = ? AND mission_id = ?", beautyCenterID, missionID).First(&progress).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &progress, nil
}

func (r *TokenMissionRepositoryImpl) FindProgressByCenter(beautyCenterID uint) ([]models.TokenMissionProgress, error) {
	var progress []models.TokenMissionProgress
	err := r.db.Where("beauty_center_id = ?", beautyCenterID).Preload("Mission").Find(&progress).Error
	return progress, err
}
