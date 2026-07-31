package repository

import "github.com/Nina-99/yottu-api/models"

type TokenMissionRepository interface {
	CreateMission(mission *models.TokenMission) error
	UpdateMission(mission *models.TokenMission) error
	FindMissionByID(id uint) (*models.TokenMission, error)
	FindActiveMissions() ([]models.TokenMission, error)
	FindMissionByType(triggerType string) (*models.TokenMission, error)
	UpsertProgress(progress *models.TokenMissionProgress) error
	FindProgress(beautyCenterID, missionID uint) (*models.TokenMissionProgress, error)
	FindProgressByCenter(beautyCenterID uint) ([]models.TokenMissionProgress, error)
}
