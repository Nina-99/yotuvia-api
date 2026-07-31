package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
)

type PersonnelInvitationServiceImpl struct {
	InvitationRepo repository.PersonnelInvitationRepository
	PersonnelRepo  repository.PersonnelRepository
	CenterRepo     repository.BeautyCenterRepository
	AuthRepo       repository.AuthAccountRepository
	Validate       *validator.Validate
}

func NewPersonnelInvitationServiceImpl(
	invitationRepo repository.PersonnelInvitationRepository,
	personnelRepo repository.PersonnelRepository,
	centerRepo repository.BeautyCenterRepository,
	authRepo repository.AuthAccountRepository,
	validate *validator.Validate,
) PersonnelInvitationService {
	return &PersonnelInvitationServiceImpl{
		InvitationRepo: invitationRepo,
		PersonnelRepo:  personnelRepo,
		CenterRepo:     centerRepo,
		AuthRepo:       authRepo,
		Validate:       validate,
	}
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

	// Hash password
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return err
	}

	// Check if personnel already exists with this email
	existingPersonnel, _ := s.PersonnelRepo.FindByEmail(invitation.Email)
	if existingPersonnel != nil {
		// Personnel exists but may not have an auth account — update their center
		existingPersonnel.BeautyCenterID = invitation.BeautyCenterID
		existingPersonnel.FirstName = firstName
		existingPersonnel.LastName = lastName
		existingPersonnel.Phone = phone
		existingPersonnel.Password = hashedPassword
		existingPersonnel.IsActive = true
		if err := s.PersonnelRepo.Update(existingPersonnel); err != nil {
			return err
		}
	} else {
		// Create new personnel
		personnel := models.Personnel{
			BeautyCenterID: invitation.BeautyCenterID,
			FirstName:      firstName,
			LastName:       lastName,
			Email:          invitation.Email,
			Phone:          phone,
			Password:       hashedPassword,
			IsActive:       true,
		}
		if err := s.PersonnelRepo.Create(&personnel); err != nil {
			return err
		}
	}

	// Mark invitation as accepted
	now := time.Now()
	invitation.Status = "accepted"
	invitation.UsedAt = &now
	if err := s.InvitationRepo.Update(invitation); err != nil {
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
