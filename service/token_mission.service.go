package service

import "github.com/Nina-99/yotuvia-api/models"

type TokenMissionService interface {
	TriggerCheck(beautyCenterID uint, triggerType string) error
	ClaimReward(beautyCenterID, missionID uint) error
	GetProgress(beautyCenterID uint) ([]models.TokenMissionProgress, error)
	GetAvailableMissions() ([]models.TokenMission, error)
}
