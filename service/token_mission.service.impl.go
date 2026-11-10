package service

import (
	"errors"
	"time"

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
	// Now is the clock used to resolve period keys. Defaults to time.Now; tests
	// can override it to simulate week/month rollover.
	Now func() time.Time
}

// clock resolves the current time, falling back to time.Now.
func (s *TokenMissionServiceImpl) clock() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
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

	// Resolve the CURRENT window for each mission: a rolled-over weekly/monthly
	// mission starts at 0 while the previous window stays as history.
	progress := make([]models.TokenMissionProgress, 0, len(missions))
	for _, mission := range missions {
		row, err := s.missionRepo.FindOrCreateProgress(&models.TokenMissionProgress{
			BeautyCenterID: beautyCenterID,
			MissionID:      mission.ID,
			PeriodKey:      MissionPeriodKey(mission.Period, s.clock()),
			CurrentCount:   0,
			TargetCount:    mission.TriggerCount,
		})
		if err != nil {
			continue
		}
		progress = append(progress, *row)
	}
	return progress, nil
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
		progress, err := s.missionRepo.FindOrCreateProgress(&models.TokenMissionProgress{
			BeautyCenterID: beautyCenterID,
			MissionID:      mission.ID,
			PeriodKey:      MissionPeriodKey(mission.Period, s.clock()),
			CurrentCount:   0,
			TargetCount:    mission.TriggerCount,
		})
		if err != nil {
			return err
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

	progress, err := s.missionRepo.FindProgress(beautyCenterID, missionID, MissionPeriodKey(mission.Period, s.clock()))
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
