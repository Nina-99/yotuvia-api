package service

import (
	"errors"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"gorm.io/gorm"
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

// TriggerCheck advances EVERY active mission matching the trigger type (D1 —
// the 1/10/50 reservation ladder shares one trigger and must co-progress).
// Progress rows are created on demand; the count increment is a single atomic
// statement (D2) so concurrent triggers can neither overshoot the target nor
// double-stamp completion.
func (s *TokenMissionServiceImpl) TriggerCheck(beautyCenterID uint, triggerType string) error {
	missions, err := s.missionRepo.FindActiveMissionsByType(triggerType)
	if err != nil {
		return err
	}
	if len(missions) == 0 {
		return nil // no mission configured for this trigger type
	}

	for _, mission := range missions {
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
			if err := s.missionRepo.UpsertProgress(progress); err != nil {
				return err
			}
		}
		if progress.CompletedAt != nil {
			continue // already completed — frozen until claim
		}
		if err := s.missionRepo.IncrementProgress(progress.ID, mission.TriggerCount); err != nil {
			return err
		}
		utils.Log.Debugf("[MISSION] Center %d progressed mission %q", beautyCenterID, mission.Name)
	}

	return nil
}

// ClaimReward credits the mission reward atomically with the claim flag (D7):
// one transaction locks the progress row, flips reward_claimed under a guard
// that rejects already-claimed or incomplete rows, and performs the token
// credit + ledger insert via AddTokensInTx. A rejected guard surfaces
// apperrors.ErrConflict so the API layer maps it to 409.
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
		// No progress row = mission never started → the claim guard treats it
		// exactly like an incomplete mission (conflict, D7).
		return apperrors.ErrConflict
	}

	var account *models.TokenAccount
	account, err = s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return err
	}
	if account == nil {
		return errors.New("token account not found")
	}

	var refPtr *uint
	if mission.ID != 0 {
		ref := mission.ID
		refPtr = &ref
	}

	return s.missionRepo.Transact(func(tx *gorm.DB) error {
		rows, err := s.missionRepo.MarkClaimed(tx, progress.ID)
		if err != nil {
			return err
		}
		if rows == 0 {
			return apperrors.ErrConflict // already claimed or incomplete
		}

		_, err = s.accountRepo.AddTokensInTx(tx, account.ID, mission.RewardTokens,
			"mission", "Mission reward: "+mission.Name, refPtr)
		return err
	})
}
