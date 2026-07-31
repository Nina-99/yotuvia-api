package configs

import (
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/utils"
)

func SeedDefaultMissions() {
	missions := []models.TokenMission{
		{
			Name:         "First Completed Reservation",
			Description:  "Complete your first reservation",
			TriggerType:  "reservation_completed",
			TriggerCount: 1,
			RewardTokens: 10,
			Icon:         "🎯",
			IsActive:     true,
			SortOrder:    1,
		},
		{
			Name:         "Loyalty Achiever",
			Description:  "Complete 50 reservations in a month",
			TriggerType:  "reservation_completed",
			TriggerCount: 50,
			RewardTokens: 25,
			Icon:         "🏆",
			IsActive:     true,
			SortOrder:    2,
		},
		{
			Name:         "Stamp Promoter",
			Description:  "Activate a stamp card promotion",
			TriggerType:  "stamp_activated",
			TriggerCount: 1,
			RewardTokens: 15,
			Icon:         "🎟️",
			IsActive:     true,
			SortOrder:    3,
		},
		{
			Name:         "Profile Star",
			Description:  "Complete your profile with photo and description",
			TriggerType:  "profile_completed",
			TriggerCount: 1,
			RewardTokens: 5,
			Icon:         "⭐",
			IsActive:     true,
			SortOrder:    4,
		},
		{
			Name:         "Review Responder",
			Description:  "Respond to 5 reviews",
			TriggerType:  "review_responded",
			TriggerCount: 5,
			RewardTokens: 3,
			Icon:         "💬",
			IsActive:     true,
			SortOrder:    5,
		},
	}

	for _, mission := range missions {
		var existing models.TokenMission
		result := DB.Where("trigger_type = ? AND trigger_count = ?", mission.TriggerType, mission.TriggerCount).First(&existing)
		if result.Error != nil {
			if err := DB.Create(&mission).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to create mission %s: %v", mission.Name, err)
			} else {
				utils.Log.Infof("[SEED] Created mission: %s (+%.0f YOTTUs)", mission.Name, mission.RewardTokens)
			}
		}
	}
}
