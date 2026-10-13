package repository

import (
	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type TokenMissionRepository interface {
	CreateMission(mission *models.TokenMission) error
	UpdateMission(mission *models.TokenMission) error
	FindMissionByID(id uint) (*models.TokenMission, error)
	FindActiveMissions() ([]models.TokenMission, error)
	// FindActiveMissionsByType returns EVERY active mission for a trigger
	// type — several missions may share one trigger (e.g. the 1 and the 50
	// reservation ladder) and all of them must progress together.
	FindActiveMissionsByType(triggerType string) ([]models.TokenMission, error)
	UpsertProgress(progress *models.TokenMissionProgress) error
	FindProgress(beautyCenterID, missionID uint) (*models.TokenMissionProgress, error)
	FindProgressByCenter(beautyCenterID uint) ([]models.TokenMissionProgress, error)
	// IncrementProgress advances current_count by exactly one step toward
	// target_count in a single atomic statement (D2): LEAST caps overshoot,
	// completed_at freezes once set. No-op when already completed.
	IncrementProgress(progressID uint, targetCount int) error
	// Transact runs fn inside the repository's database transaction so the
	// service can compose the claim's flag update and token credit atomically.
	Transact(fn func(tx *gorm.DB) error) error
	// MarkClaimed flips reward_claimed inside a caller-managed transaction,
	// guarded by completion + not-yet-claimed. Returns rows affected: zero
	// means the guard rejected the update (already claimed or incomplete).
	MarkClaimed(tx *gorm.DB, progressID uint) (int64, error)
}
