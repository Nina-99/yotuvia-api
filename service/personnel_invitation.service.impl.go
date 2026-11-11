package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

// errInvitationTaken aborts the accept transaction when a concurrent request
// already claimed the invitation (guarded UPDATE affected zero rows).
var errInvitationTaken = errors.New("invitation already accepted")

type PersonnelInvitationServiceImpl struct {
	InvitationRepo repository.PersonnelInvitationRepository
	PersonnelRepo  repository.PersonnelRepository
	CenterRepo     repository.BeautyCenterRepository
	ClientRepo     repository.ClientRepository
	AuthRepo       repository.AuthAccountRepository
	Validate       *validator.Validate
}

func NewPersonnelInvitationServiceImpl(
	invitationRepo repository.PersonnelInvitationRepository,
	personnelRepo repository.PersonnelRepository,
	centerRepo repository.BeautyCenterRepository,
	clientRepo repository.ClientRepository,
	authRepo repository.AuthAccountRepository,
	validate *validator.Validate,
) PersonnelInvitationService {
	return &PersonnelInvitationServiceImpl{
		InvitationRepo: invitationRepo,
		PersonnelRepo:  personnelRepo,
		CenterRepo:     centerRepo,
		ClientRepo:     clientRepo,
		AuthRepo:       authRepo,
		Validate:       validate,
	}
}

// emailBelongsToOtherRole enforces one account per email: a client or centre
// email cannot be invited as staff.
func (s *PersonnelInvitationServiceImpl) emailBelongsToOtherRole(email string) (bool, error) {
	if client, err := s.ClientRepo.FindByEmail(email); err != nil {
		return false, err
	} else if client != nil {
		return true, nil
	}
	if center, err := s.CenterRepo.FindByEmail(email); err != nil {
		return false, err
	} else if center != nil {
		return true, nil
	}
	return false, nil
}

// ──────────────────────────────────────────────
//  GENERATE — Crea un token de invitación
// ──────────────────────────────────────────────

func generateInviteToken() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *PersonnelInvitationServiceImpl) Generate(beautyCenterID uint, email string) (string, error) {
	// Validate center exists
	center, err := s.CenterRepo.FindByID(beautyCenterID)
	if err != nil || center == nil {
		return "", errors.New("beauty center not found")
	}
	// One account per email: a client or another centre cannot be invited as staff.
	if taken, err := s.emailBelongsToOtherRole(email); err != nil {
		return "", err
	} else if taken {
		return "", errors.New("email already registered with another account")
	}

	// If there's a pending invitation, renew it instead of rejecting
	existing, _ := s.InvitationRepo.FindByEmailAndCenter(email, beautyCenterID)
	if existing != nil && existing.Status == "pending" {
		newToken, err := generateInviteToken()
		if err != nil {
			return "", err
		}
		existing.Token = newToken
		existing.ExpiresAt = time.Now().Add(72 * time.Hour)
		if err := s.InvitationRepo.Update(existing); err != nil {
			return "", err
		}
		utils.Log.Infof("[INVITE] Invitation renewed for %s by center %d, token=%s", email, beautyCenterID, newToken[:8])
		return newToken, nil
	}

	// Check if personnel with this email already exists for this center
	existingPersonnel, _ := s.PersonnelRepo.FindByEmail(email)
	if existingPersonnel != nil && existingPersonnel.BeautyCenterID == beautyCenterID {
		return "", errors.New("a personnel with this email already belongs to this center")
	}

	token, err := generateInviteToken()
	if err != nil {
		return "", err
	}

	invitation := models.PersonnelInvitation{
		BeautyCenterID: beautyCenterID,
		Email:          email,
		Token:          token,
		ExpiresAt:      time.Now().Add(72 * time.Hour), // 3 days
		Status:         "pending",
	}

	if err := s.InvitationRepo.Create(&invitation); err != nil {
		return "", err
	}

	utils.Log.Infof("[INVITE] Invitation generated for %s by center %d, token=%s", email, beautyCenterID, token[:8])
	return token, nil
}

// ──────────────────────────────────────────────
//  GET BY TOKEN — Obtiene info de la invitación
// ──────────────────────────────────────────────

func (s *PersonnelInvitationServiceImpl) GetByToken(token string) (*response.InvitationResponse, error) {
	invitation, err := s.InvitationRepo.FindByToken(token)
	if err != nil || invitation == nil {
		return nil, errors.New("invitation not found")
	}

	if invitation.Status != "pending" {
		return nil, errors.New("invitation is no longer valid")
	}

	if time.Now().After(invitation.ExpiresAt) {
		invitation.Status = "expired"
		_ = s.InvitationRepo.Update(invitation)
		return nil, errors.New("invitation has expired")
	}

	center, _ := s.CenterRepo.FindByID(invitation.BeautyCenterID)
	centerName := ""
	if center != nil {
		centerName = center.Name
	}

	return &response.InvitationResponse{
		Token:          invitation.Token,
		Email:          invitation.Email,
		BeautyCenterID: invitation.BeautyCenterID,
		CenterName:     centerName,
		ExpiresAt:      invitation.ExpiresAt,
	}, nil
}

// ──────────────────────────────────────────────
//  ACCEPT — Acepta la invitación y crea el personnel
// ──────────────────────────────────────────────

func (s *PersonnelInvitationServiceImpl) Accept(token, firstName, lastName, phone, password string) error {
	invitation, err := s.InvitationRepo.FindByToken(token)
	if err != nil || invitation == nil {
		return errors.New("invitation not found")
	}

	if invitation.Status != "pending" {
		return errors.New("invitation is no longer valid")
	}

	if time.Now().After(invitation.ExpiresAt) {
		invitation.Status = "expired"
		_ = s.InvitationRepo.Update(invitation)
		return errors.New("invitation has expired")
	}

	// Validate required fields
	if firstName == "" || lastName == "" || password == "" {
		return errors.New("first_name, last_name, and password are required")
	}

	// One account per email: an invited email must not already be a client or a
	// centre. (An existing staff row for this email is the intended invitee.)
	if taken, err := s.emailBelongsToOtherRole(invitation.Email); err != nil {
		return err
	} else if taken {
		return errors.New("email already registered with another account")
	}

	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return err
	}

	existingPersonnel, _ := s.PersonnelRepo.FindByEmail(invitation.Email)
	now := time.Now()

	// Claim the invitation, write the staff row and link the local AuthAccount
	// in ONE transaction. The guarded UPDATE is the concurrency control: only
	// the request that flips pending->accepted may create the staff account, so
	// two simultaneous accepts cannot both succeed.
	err = s.InvitationRepo.Transact(func(tx *gorm.DB) error {
		res := tx.Model(&models.PersonnelInvitation{}).
			Where("id = ? AND status = ?", invitation.ID, "pending").
			Updates(map[string]any{"status": "accepted", "used_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errInvitationTaken
		}

		var personnel models.Personnel
		if existingPersonnel != nil {
			personnel = *existingPersonnel
			personnel.BeautyCenterID = invitation.BeautyCenterID
			personnel.FirstName = firstName
			personnel.LastName = lastName
			personnel.Phone = phone
			personnel.Password = hashedPassword
			personnel.IsActive = true
			if err := tx.Save(&personnel).Error; err != nil {
				return err
			}
		} else {
			personnel = models.Personnel{
				BeautyCenterID: invitation.BeautyCenterID,
				FirstName:      firstName,
				LastName:       lastName,
				Email:          invitation.Email,
				Phone:          phone,
				Password:       hashedPassword,
				IsActive:       true,
			}
			if err := tx.Create(&personnel).Error; err != nil {
				return err
			}
		}

		// Ensure a local AuthAccount exists so password recovery works for staff
		// that were invited (rather than created by the owner).
		var account models.AuthAccount
		findErr := tx.Where("provider = ? AND user_type = ? AND personnel_id = ?", "local", "personnel", personnel.ID).
			First(&account).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			account = models.AuthAccount{
				Provider:    "local",
				ProviderID:  personnel.Email,
				UserType:    "personnel",
				PersonnelID: &personnel.ID,
			}
			if err := tx.Create(&account).Error; err != nil {
				return err
			}
		} else if findErr != nil {
			return findErr
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errInvitationTaken) {
			return errors.New("invitation is no longer valid")
		}
		return err
	}

	utils.Log.Infof("[INVITE] Invitation %s accepted by %s %s", token[:8], firstName, lastName)
	return nil
}

// ──────────────────────────────────────────────
//  LIST — Lista invitaciones de un centro
// ──────────────────────────────────────────────

func (s *PersonnelInvitationServiceImpl) ListByCenter(beautyCenterID uint) ([]response.InvitationResponse, error) {
	invitations, err := s.InvitationRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return nil, err
	}

	center, _ := s.CenterRepo.FindByID(beautyCenterID)
	centerName := ""
	if center != nil {
		centerName = center.Name
	}

	result := make([]response.InvitationResponse, len(invitations))
	for i, inv := range invitations {
		result[i] = response.InvitationResponse{
			ID:             inv.ID,
			Token:          inv.Token,
			Email:          inv.Email,
			BeautyCenterID: inv.BeautyCenterID,
			CenterName:     centerName,
			Status:         inv.Status,
			ExpiresAt:      inv.ExpiresAt,
			UsedAt:         inv.UsedAt,
			CreatedAt:      inv.CreatedAt,
		}
	}
	return result, nil
}
