package models

import "gorm.io/gorm"

type TokenMission struct {
	gorm.Model
	Name         string `gorm:"size:100;not null" json:"name"`
	Description  string `gorm:"size:255" json:"description"`
	TriggerType  string `gorm:"size:50;not null" json:"trigger_type"` // reservation_completed, profile_completed, review_responded, referral
	TriggerCount int    `gorm:"not null" json:"trigger_count"`        // how many times to trigger
	RewardTokens int64  `gorm:"type:bigint;not null" json:"reward_tokens"`
	Icon         string `gorm:"size:50" json:"icon"`
	IsActive     bool   `gorm:"default:true" json:"is_active"`
	SortOrder    int    `gorm:"default:0" json:"sort_order"`
}
