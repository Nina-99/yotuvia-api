package service

import (
	"errors"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
)

type TokenMissionServiceImpl struct {
	missionRepo repository.TokenMissionRepository
	accountSvc  TokenAccountService
	accountRepo repository.TokenAccountRepository
}

func NewTokenMissionServiceImpl(
	missionRepo repository.TokenMissionRepository,
	accountSvc TokenAccountService,
	accountRepo repository.TokenAccountRepository,
) TokenMissionService {
	return &TokenMissionServiceImpl{
		missionRepo: missionRepo,
		accountSvc:  accountSvc,
		accountRepo: accountRepo,
	}
}

func (s *TokenMissionServiceImpl) GetAvailableMissions() ([]models.TokenMission, error) {
	return s.missionRepo.FindActiveMissions()
}

func (s *TokenMissionServiceImpl) GetProgress(beautyCenterID uint) ([]models.TokenMissionProgress, error) {
	missions, err := s.missionRepo.FindActiveMissions()
	if err != nil {
		return nil, err
	}

	// Ensure progress exists for all active missions
	for _, mission := range missions {
		existing, err := s.missionRepo.FindProgress(beautyCenterID, mission.ID)
		if err != nil {
			continue
		}
		if existing == nil {
			_ = s.missionRepo.UpsertProgress(&models.TokenMissionProgress{
				BeautyCenterID: beautyCenterID,
				MissionID:      mission.ID,
				CurrentCount:   0,
				TargetCount:    mission.TriggerCount,
			})
		}
	}

	return s.missionRepo.FindProgressByCenter(beautyCenterID)
}

func (s *TokenMissionServiceImpl) TriggerCheck(beautyCenterID uint, triggerType string) error {
	mission, err := s.missionRepo.FindMissionByType(triggerType)
	if err != nil {
		return err
	}
	if mission == nil {
		return nil // no mission configured for this trigger type
	}

	progress, err := s.missionRepo.FindProgress(beautyCenterID, mission.ID)
	if err != nil {
		return err
	}

	if progress == nil {
		progress = &models.TokenMissionProgress{
			BeautyCenterID: beautyCenterID,
			MissionID:      mission.ID,
			CurrentCount:   0,
			TargetCount:    mission.TriggerCount,
		}
	}

	if progress.CompletedAt != nil {
		return nil // already completed
	}

	progress.CurrentCount++
	if err := s.missionRepo.UpsertProgress(progress); err != nil {
		return err
	}

	utils.Log.Debugf("[MISSION] Center %d progressed mission %s: %d/%d",
		beautyCenterID, mission.Name, progress.CurrentCount, progress.TargetCount)

	return nil
}

func (s *TokenMissionServiceImpl) ClaimReward(beautyCenterID, missionID uint) error {
	mission, err := s.missionRepo.FindMissionByID(missionID)
	if err != nil {
		return err
	}
	if mission == nil {
		return errors.New("mission not found")
	}

	progress, err := s.missionRepo.FindProgress(beautyCenterID, missionID)
	if err != nil {
		return err
	}
	if progress == nil {
		return errors.New("mission progress not found")
	}

	if progress.RewardClaimed {
		return errors.New("reward already claimed")
	}

	if progress.CurrentCount < progress.TargetCount {
		return errors.New("mission not yet completed")
	}

	// Mark as completed and claimed
	now := time.Now()
	progress.CompletedAt = &now
	progress.RewardClaimed = true

	if err := s.missionRepo.UpsertProgress(progress); err != nil {
		return err
	}

	// Award tokens — RewardTokens is already milli-YOTTUs, passed through
	// unchanged to the atomic AddTokens.
	if err := s.accountSvc.AddTokens(beautyCenterID, mission.RewardTokens, "mission", mission.ID, "Mission reward: "+mission.Name); err != nil {
		return err
	}

	utils.Log.Infof("[MISSION] Center %d claimed reward for %s: +%.2f YOTTUs", beautyCenterID, mission.Name, utils.FromMilli(mission.RewardTokens))
	return nil
}
