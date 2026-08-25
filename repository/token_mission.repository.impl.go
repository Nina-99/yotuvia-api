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

// FindActiveMissionsByType returns every active mission for the trigger type.
// Several missions share one trigger (the 1/10/50 reservation ladder) and all
// of them must progress together (design decision D1).
func (r *TokenMissionRepositoryImpl) FindActiveMissionsByType(triggerType string) ([]models.TokenMission, error) {
	var missions []models.TokenMission
	err := r.db.Where("trigger_type = ? AND is_active = ?", triggerType, true).
		Order("trigger_count ASC, id ASC").
		Find(&missions).Error
	return missions, err
}

// IncrementProgress advances one step atomically in a single statement (D2):
// a standard-SQL CASE caps overshoot under concurrent triggers and COALESCE
// stamps completed_at exactly once. The completed_at IS NULL guard freezes
// missions that already finished.
func (r *TokenMissionRepositoryImpl) IncrementProgress(progressID uint, targetCount int) error {
	err := r.db.Exec(`
		UPDATE token_mission_progresses
		SET current_count = CASE
		        WHEN current_count + 1 < target_count THEN current_count + 1
		        ELSE target_count END,
		    completed_at = COALESCE(completed_at,
		        CASE WHEN current_count + 1 >= target_count THEN CURRENT_TIMESTAMP ELSE completed_at END)
		WHERE id = ? AND completed_at IS NULL`,
		progressID).Error
	return err
}

// Transact exposes the repository's database transaction so the service can
// compose claim flag + token credit atomically (D7).
func (r *TokenMissionRepositoryImpl) Transact(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// MarkClaimed flips reward_claimed only when the row is complete and not yet
// claimed. Runs inside the caller's transaction; rows affected zero signals
// a rejected guard (conflict at the service layer).
func (r *TokenMissionRepositoryImpl) MarkClaimed(tx *gorm.DB, progressID uint) (int64, error) {
	res := tx.Model(&models.TokenMissionProgress{}).
		Where("id = ? AND reward_claimed = ? AND current_count >= target_count", progressID, false).
		Update("reward_claimed", true)
	return res.RowsAffected, res.Error
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
