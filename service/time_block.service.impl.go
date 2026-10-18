package service

import (
	"strings"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/go-playground/validator/v10"
)

type TimeBlockServiceImpl struct {
	repo          repository.TimeBlockRepository
	personnelRepo repository.PersonnelRepository
	validate      *validator.Validate
}

func NewTimeBlockService(
	repo repository.TimeBlockRepository,
	personnelRepo repository.PersonnelRepository,
	validate *validator.Validate,
) TimeBlockService {
	return &TimeBlockServiceImpl{repo: repo, personnelRepo: personnelRepo, validate: validate}
}

func (s *TimeBlockServiceImpl) Create(req request.CreateTimeBlockRequest, role string, actorID, centerID uint) (*models.TimeBlock, error) {
	if err := s.validate.Struct(req); err != nil {
		return nil, apperrors.ErrValidation
	}

	day, err := time.Parse("2006-01-02", strings.TrimSpace(req.Date))
	if err != nil {
		return nil, apperrors.ErrBadRequest
	}
	start := models.ParseClockMinutes(req.StartTime)
	end := models.ParseClockMinutes(req.EndTime)
	if start <= 0 && end <= 0 || start >= end {
		// A zero/zero window is meaningless and an inverted one is a typo.
		return nil, apperrors.ErrBadRequest
	}

	block := &models.TimeBlock{
		BeautyCenterID: centerID,
		Date:           day,
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		Reason:         strings.TrimSpace(req.Reason),
		CreatedByRole:  role,
		CreatedByID:    actorID,
	}

	switch role {
	case "personnel":
		// A professional blocks only their own time: the body cannot name
		// another professional.
		if req.PersonnelID != nil && *req.PersonnelID != actorID {
			return nil, apperrors.ErrForbidden
		}
		own := actorID
		block.PersonnelID = &own
	case "beauty_center":
		if req.PersonnelID != nil {
			personnel, err := s.personnelRepo.FindByID(*req.PersonnelID)
			if err != nil || personnel == nil {
				return nil, apperrors.ErrNotFound
			}
			if personnel.BeautyCenterID != centerID {
				return nil, apperrors.ErrForbidden
			}
		}
		block.PersonnelID = req.PersonnelID // nil = centre-wide
	default:
		return nil, apperrors.ErrForbidden
	}

	if err := s.repo.Create(block); err != nil {
		return nil, err
	}
	return block, nil
}

func (s *TimeBlockServiceImpl) List(role string, actorID, centerID uint, from, to string, personnelID *uint) ([]models.TimeBlock, error) {
	fromDay, err := time.Parse("2006-01-02", strings.TrimSpace(from))
	if err != nil {
		return nil, apperrors.ErrBadRequest
	}
	toDay, err := time.Parse("2006-01-02", strings.TrimSpace(to))
	if err != nil {
		return nil, apperrors.ErrBadRequest
	}
	if toDay.Before(fromDay) {
		return nil, apperrors.ErrBadRequest
	}

	blocks, err := s.repo.ListByCentersRange([]uint{centerID}, fromDay, toDay)
	if err != nil {
		return nil, err
	}

	switch role {
	case "beauty_center":
		// Optional filter by one professional (centre-wide rows always shown).
		if personnelID != nil {
			filtered := make([]models.TimeBlock, 0, len(blocks))
			for _, b := range blocks {
				if b.PersonnelID == nil || *b.PersonnelID == *personnelID {
					filtered = append(filtered, b)
				}
			}
			return filtered, nil
		}
		return blocks, nil
	case "personnel":
		// A professional sees their own blocks plus the centre-wide ones that
		// affect them.
		filtered := make([]models.TimeBlock, 0, len(blocks))
		for _, b := range blocks {
			if b.PersonnelID == nil || *b.PersonnelID == actorID {
				filtered = append(filtered, b)
			}
		}
		return filtered, nil
	default:
		return nil, apperrors.ErrForbidden
	}
}

func (s *TimeBlockServiceImpl) Delete(id uint, role string, actorID, centerID uint) error {
	block, err := s.repo.FindByID(id)
	if err != nil || block == nil {
		return apperrors.ErrNotFound
	}
	if block.BeautyCenterID != centerID {
		return apperrors.ErrForbidden
	}

	switch role {
	case "beauty_center":
		// The owner may remove any block of its centre.
	case "personnel":
		// A professional may remove only the blocks they created.
		if block.CreatedByRole != "personnel" || block.CreatedByID != actorID {
			return apperrors.ErrForbidden
		}
	default:
		return apperrors.ErrForbidden
	}

	return s.repo.Delete(id)
}
