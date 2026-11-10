package models

import (
	"time"

	"gorm.io/gorm"
)

type TokenMissionProgress struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	BeautyCenterID uint         `gorm:"not null;uniqueIndex:idx_mission_center_period" json:"beauty_center_id"`
	BeautyCenter   BeautyCenter `gorm:"foreignKey:BeautyCenterID" json:"beauty_center,omitempty"`
	MissionID      uint         `gorm:"not null;uniqueIndex:idx_mission_center_period" json:"mission_id"`
	Mission        TokenMission `gorm:"foreignKey:MissionID" json:"mission,omitempty"`
	// PeriodKey scopes the row to one mission window: "once", "2026-W45"
	// (weekly) or "2026-11" (monthly). A new period starts a fresh row.
	PeriodKey string `gorm:"size:16;not null;default:'once';uniqueIndex:idx_mission_center_period" json:"period_key"`

	CurrentCount  int        `gorm:"default:0" json:"current_count"`
	TargetCount   int        `gorm:"not null" json:"target_count"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	RewardClaimed bool       `gorm:"default:false" json:"reward_claimed"`
}
