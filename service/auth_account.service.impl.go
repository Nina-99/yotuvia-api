package service

import (
	"context"
	"errors"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
)

type AuthAccountServiceImpl struct {
	AuthRepo        repository.AuthAccountRepository
	ClientRepo      repository.ClientRepository
	PersonnelRepo   repository.PersonnelRepository
	CenterRepo      repository.BeautyCenterRepository
	SessionRepo     repository.UserSessionRepository
	TokenAccountSvc TokenAccountService
	Validate        *validator.Validate
}

func NewAuthAccountServiceImpl(
	authRepo repository.AuthAccountRepository,
	clientRepo repository.ClientRepository,
	personnelRepo repository.PersonnelRepository,
	centerRepo repository.BeautyCenterRepository,
	sessionRepo repository.UserSessionRepository,
	tokenAccountSvc TokenAccountService,
	validate *validator.Validate,
) AuthAccountService {
	return &AuthAccountServiceImpl{
		AuthRepo:        authRepo,
		ClientRepo:      clientRepo,
		PersonnelRepo:   personnelRepo,
		CenterRepo:      centerRepo,
		SessionRepo:     sessionRepo,
		TokenAccountSvc: tokenAccountSvc,
		Validate:        validate,
	}
}

// mintSession creates a new active session generation for the authenticated
// user and returns the raw refresh token to hand to the client exactly once.
// Sessions key on the polymorphic JWT user_id claim (D1).
func (s *AuthAccountServiceImpl) mintSession(userID, centerID uint, role, email, username string) (string, error) {
	raw, err := utils.GenerateRefreshToken()
	if err != nil {
		return "", err
	}
	familyID, err := utils.GenerateFamilyID()
	if err != nil {
		return "", err
	}
	session := &models.UserSession{
		UserID:           userID,
		FamilyID:         familyID,
		RefreshTokenHash: utils.HashRefreshToken(raw),
		ExpiresAt:        time.Now().Add(utils.RefreshTokenTTL),
		Role:             role,
		CenterID:         centerID,
		Email:            email,
		Username:         username,
	}
	if err := s.SessionRepo.Create(context.Background(), session); err != nil {
		return "", err
	}
	return raw, nil
}

// ──────────────────────────────────────────────
//  CHECK EMAIL
// ──────────────────────────────────────────────

func (s *AuthAccountServiceImpl) CheckEmail(req request.CheckEmailRequest) (bool, error) {
	if err := s.Validate.Struct(req); err != nil {
		return false, err
	}

	client, _ := s.ClientRepo.FindByEmail(req.Email)
	if client != nil {
		return true, nil
	}

	center, _ := s.CenterRepo.FindByEmail(req.Email)
	if center != nil {
		return true, nil
	}

	personnel, _ := s.PersonnelRepo.FindByEmail(req.Email)
	if personnel != nil {
		return true, nil
	}

	return false, nil
}

// ──────────────────────────────────────────────
//  LOGIN
// ──────────────────────────────────────────────

func (s *AuthAccountServiceImpl) Login(req request.LoginRequest) (AuthResult, error) {
	if err := s.Validate.Struct(req); err != nil {
		return AuthResult{}, err
	}

	if req.UserType != "" {
		return s.loginByType(req.Identifier, req.Password, req.UserType)
	}

	result, err := s.tryLoginAllTypes(req.Identifier, req.Password)
	if err != nil {
		return AuthResult{}, err
	}

	return result, nil
}

func (s *AuthAccountServiceImpl) loginByType(identifier, password, userType string) (AuthResult, error) {
	var userID uint
	var centerID uint
	var email string
	var username string
	var firstName string
	var lastName string
	var passwordHash string

	switch userType {
	case "client":
		var client *models.Client
		var err error
		if utils.IsEmail(identifier) {
			client, err = s.ClientRepo.FindByEmail(identifier)
		} else {
			client, err = s.ClientRepo.FindByUsername(identifier)
		}
		if err != nil || client == nil {
			return AuthResult{}, errors.New("invalid credentials")
		}
		userID = client.ID
		centerID = 0
		email = client.Email
		username = client.Username
		firstName = client.FirstName
		lastName = client.LastName
		passwordHash = client.Password

	case "personnel":
		personnel, err := s.PersonnelRepo.FindByEmail(identifier)
		if err != nil || personnel == nil {
			return AuthResult{}, errors.New("invalid credentials")
		}
		userID = personnel.ID
		centerID = personnel.BeautyCenterID
		email = personnel.Email
		username = personnel.FirstName
		firstName = personnel.FirstName
		lastName = personnel.LastName
		passwordHash = personnel.Password

	case "beauty_center":
		center, err := s.CenterRepo.FindByEmail(identifier)
		if err != nil || center == nil {
			return AuthResult{}, errors.New("invalid credentials")
		}
		userID = center.ID
		centerID = center.ID
		email = center.Email
		username = center.Name
		firstName = center.Name
		lastName = ""
		passwordHash = center.Password
	}

	if passwordHash == "" {
		return AuthResult{}, errors.New("please log in using your social account or set a password first")
	}

	if !utils.VerifyPassword(password, passwordHash) {
		return AuthResult{}, errors.New("invalid credentials")
	}

	utils.Log.Infof("[AUTH] Classic Login successful. UserType: %s, ID: %d", userType, userID)
	token, err := utils.GenerateJWT(userID, centerID, userType, email, username)
	if err != nil {
		return AuthResult{}, err
	}

	refreshToken, err := s.mintSession(userID, centerID, userType, email, username)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		Token:        token,
		RefreshToken: refreshToken,
		UserID:       userID,
		Email:        email,
		FirstName:    firstName,
		LastName:     lastName,
		Role:         userType,
	}, nil
}

func (s *AuthAccountServiceImpl) tryLoginAllTypes(identifier, password string) (AuthResult, error) {
	types := []string{"client", "beauty_center", "personnel"}

	var lastErr error
	for _, userType := range types {
		result, err := s.loginByType(identifier, password, userType)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	utils.Log.Warnf("[AUTH] Login failed for %s: %v", identifier, lastErr)
	return AuthResult{}, errors.New("invalid credentials")
}

// ──────────────────────────────────────────────
//  REGISTER
// ──────────────────────────────────────────────

func (s *AuthAccountServiceImpl) Register(req request.RegisterRequest) (AuthResult, error) {
	if err := s.Validate.Struct(req); err != nil {
		return AuthResult{}, err
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return AuthResult{}, err
	}

	var userID uint
	var centerID uint
	var email string
	var username string
	var firstName string
	var lastName string

	switch req.UserType {
	case "client":
		existingEmail, _ := s.ClientRepo.FindByEmail(req.Email)
		if existingEmail != nil {
			return AuthResult{}, errors.New("email already exists")
		}
		usernameVal := req.Username
		if usernameVal == "" {
			usernameVal = req.Email
		}
		existingUsername, _ := s.ClientRepo.FindByUsername(usernameVal)
		if existingUsername != nil {
			return AuthResult{}, errors.New("username already exists")
		}
		client := models.Client{
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Username:  usernameVal,
			Email:     req.Email,
			Phone:     req.Phone,
			Password:  hashedPassword,
			IsActive:  true,
		}
		if err := s.ClientRepo.Create(&client); err != nil {
			return AuthResult{}, err
		}
		userID = client.ID
		centerID = 0
		email = client.Email
		username = client.Username
		firstName = client.FirstName
		lastName = client.LastName

	case "personnel":
		existingEmail, _ := s.PersonnelRepo.FindByEmail(req.Email)
		if existingEmail != nil {
			return AuthResult{}, errors.New("email already exists")
		}
		personnel := models.Personnel{
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Email:     req.Email,
			Phone:     req.Phone,
			Password:  hashedPassword,
			IsActive:  false,
		}
		if err := s.PersonnelRepo.Create(&personnel); err != nil {
			return AuthResult{}, err
		}
		userID = personnel.ID
		centerID = personnel.BeautyCenterID
		email = personnel.Email
		username = personnel.FirstName
		firstName = personnel.FirstName
		lastName = personnel.LastName

	case "beauty_center":
		existingEmail, _ := s.CenterRepo.FindByEmail(req.Email)
		if existingEmail != nil {
			return AuthResult{}, errors.New("email already exists")
		}
		// Generate unique referral code
		refCode, err := utils.GenerateReferralCode()
		if err != nil {
			return AuthResult{}, errors.New("failed to generate referral code")
		}
		center := models.BeautyCenter{
			Name:         req.FirstName,
			Address:      req.Address,
			Email:        req.Email,
			Phone:        req.Phone,
			Password:     hashedPassword,
			Url:          req.Email,
			Latitude:     req.Latitude,
			Longitude:    req.Longitude,
			IsActive:     false,
			ReferralCode: refCode,
		}
		if err := s.CenterRepo.Create(&center); err != nil {
			return AuthResult{}, err
		}
		userID = center.ID
		centerID = center.ID
		email = center.Email
		username = center.Name
		firstName = center.Name
		lastName = ""

		// ── Welcome bonus: 25 free YOTTUs ──
		if s.TokenAccountSvc != nil {
			if _, err := s.TokenAccountSvc.GetOrCreate(center.ID, center.Currency); err != nil {
				utils.Log.Errorf("[AUTH] Failed to create token account for center %d: %v", center.ID, err)
			} else if err := s.TokenAccountSvc.AddTokens(center.ID, utils.ToMilli(25), "bonus", 0, "Welcome bonus: 25 free YOTTUs"); err != nil {
				utils.Log.Errorf("[AUTH] Failed to add welcome YOTTUs for center %d: %v", center.ID, err)
			} else {
				utils.Log.Infof("[AUTH] Welcome bonus: 25 YOTTUs credited to center %d", center.ID)
			}
		}

	default:
		return AuthResult{}, errors.New("invalid user type")
	}

	utils.Log.Infof("[AUTH] New %s registered successfully. ID: %d, Email: %s", req.UserType, userID, email)
	token, err := utils.GenerateJWT(userID, centerID, req.UserType, email, username)
	if err != nil {
		return AuthResult{}, err
	}

	refreshToken, err := s.mintSession(userID, centerID, req.UserType, email, username)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		Token:        token,
		RefreshToken: refreshToken,
		UserID:       userID,
		Email:        email,
		FirstName:    firstName,
		LastName:     lastName,
		Role:         req.UserType,
	}, nil
}

// ──────────────────────────────────────────────
//  SOCIAL LOGIN
// ──────────────────────────────────────────────

func (s *AuthAccountServiceImpl) SocialLogin(req request.SocialLoginRequest) (AuthResult, error) {
	if err := s.Validate.Struct(req); err != nil {
		return AuthResult{}, err
	}

	var providerID string
	var email string
	var firstName string
	var lastName string
	var picture string

	if req.Provider == "google" {
		userInfo, err := utils.VerifyGoogleToken(req.AccessToken)
		if err != nil {
			return AuthResult{}, err
		}
		providerID = userInfo.ID
		email = userInfo.Email
		firstName = userInfo.GivenName
		lastName = userInfo.FamilyName
		picture = userInfo.Picture
	} else {
		userInfo, err := utils.VerifyFacebookToken(req.AccessToken)
		if err != nil {
			return AuthResult{}, err
		}
		providerID = userInfo.ID
		email = userInfo.Email
		firstName = userInfo.FirstName
		lastName = userInfo.LastName
		picture = userInfo.Picture.Data.URL
	}

	account, err := s.AuthRepo.FindByProvider(req.Provider, providerID)
	if err != nil {
		return AuthResult{}, err
	}

	if account != nil {
		utils.Log.Infof("[AUTH] Social Login successful. UserType: %s, Provider: %s", account.UserType, req.Provider)
		return s.generateTokenForUser(account)
	}

	utils.Log.Infof("[AUTH] Social Account not found. Creating new %s via %s", req.UserType, req.Provider)
	newAccount := models.AuthAccount{
		Provider:   req.Provider,
		ProviderID: providerID,
		UserType:   req.UserType,
	}

	switch req.UserType {
	case "client":
		client := models.Client{
			FirstName: firstName,
			LastName:  lastName,
			Email:     email,
			Username:  email,
			AvatarURL: picture,
			IsActive:  true,
		}
		if err := s.ClientRepo.Create(&client); err != nil {
			return AuthResult{}, err
		}
		newAccount.ClientID = &client.ID

	case "personnel":
		personnel := models.Personnel{
			FirstName: firstName,
			LastName:  lastName,
			Email:     email,
			IsActive:  false,
			PersonnelImage: []models.PersonnelImage{
				{
					ImageUrl: picture,
					IsCover:  true,
					Position: 1,
				},
			},
		}
		if err := s.PersonnelRepo.Create(&personnel); err != nil {
			return AuthResult{}, err
		}
		newAccount.PersonnelID = &personnel.ID

	case "beauty_center":
		center := models.BeautyCenter{
			Name:     firstName + " " + lastName,
			Email:    email,
			IsActive: false,
			BeautyCenterImage: []models.BeautyCenterImage{
				{
					ImageUrl: picture,
					IsCover:  true,
					Position: 1,
				},
			},
		}
		if err := s.CenterRepo.Create(&center); err != nil {
			return AuthResult{}, err
		}
		newAccount.BeautyCenterID = &center.ID
	}

	if err := s.AuthRepo.Create(&newAccount); err != nil {
		return AuthResult{}, err
	}

	utils.Log.Infof("[AUTH] Created new social link and logged in. UserType: %s", req.UserType)
	return s.generateTokenForUser(&newAccount)
}

// ──────────────────────────────────────────────
//  REGISTER PERSONNEL (admin)
// ──────────────────────────────────────────────

func (s *AuthAccountServiceImpl) RegisterPersonnel(req request.RegisterPersonnelRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	// Check existing email
	existing, _ := s.PersonnelRepo.FindByEmail(req.Email)
	if existing != nil {
		return errors.New("email already exists")
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return err
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	personnel := models.Personnel{
		BeautyCenterID: req.BeautyCenterID,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Email:          req.Email,
		Phone:          req.Phone,
		Password:       hashedPassword,
		IsActive:       isActive,
	}

	utils.Log.Infof("[AUTH] New personnel registered via admin. Email: %s, CenterID: %d", req.Email, req.BeautyCenterID)
	return s.PersonnelRepo.Create(&personnel)
}

func (s *AuthAccountServiceImpl) generateTokenForUser(account *models.AuthAccount) (AuthResult, error) {
	var userID uint
	var centerID uint
	var email string
	var username string
	var firstName string
	var lastName string

	switch account.UserType {
	case "client":
		if account.ClientID != nil {
			userID = *account.ClientID
			centerID = 0
			client, _ := s.ClientRepo.FindById(userID)
			if client != nil {
				email = client.Email
				username = client.Username
				firstName = client.FirstName
				lastName = client.LastName
			}
		}
	case "personnel":
		if account.PersonnelID != nil {
			userID = *account.PersonnelID
			personnel, _ := s.PersonnelRepo.FindByID(userID)
			if personnel != nil {
				email = personnel.Email
				username = personnel.FirstName
				firstName = personnel.FirstName
				lastName = personnel.LastName
				centerID = personnel.BeautyCenterID
			}
		}
	case "beauty_center":
		if account.BeautyCenterID != nil {
			userID = *account.BeautyCenterID
			centerID = *account.BeautyCenterID
			center, _ := s.CenterRepo.FindByID(userID)
			if center != nil {
				email = center.Email
				username = center.Name
				firstName = center.Name
				lastName = ""
			}
		}
	}

	if userID == 0 {
		return AuthResult{}, errors.New("failed to identify user ID associated with this social account")
	}

	token, err := utils.GenerateJWT(userID, centerID, account.UserType, email, username)
	if err != nil {
		return AuthResult{}, err
	}

	refreshToken, err := s.mintSession(userID, centerID, account.UserType, email, username)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		Token:        token,
		RefreshToken: refreshToken,
		UserID:       userID,
		Email:        email,
		FirstName:    firstName,
		LastName:     lastName,
		Role:         account.UserType,
	}, nil
}

// ──────────────────────────────────────────────
//  REFRESH
// ──────────────────────────────────────────────

// Refresh rotates a refresh token (D2). The repository transaction atomically
// revokes the presented generation and creates the next one; reuse (a stale,
// already-rotated token) revokes the entire family. Every invalid outcome maps
// to 401 — the refresh token is the only credential this endpoint has.
func (s *AuthAccountServiceImpl) Refresh(ctx context.Context, rawToken string) (RefreshResult, error) {
	if rawToken == "" {
		return RefreshResult{}, apperrors.ErrUnauthorized
	}

	var newRaw string
	newSession, err := s.SessionRepo.Rotate(ctx, utils.HashRefreshToken(rawToken), func(active *models.UserSession) (*models.UserSession, error) {
		raw, err := utils.GenerateRefreshToken()
		if err != nil {
			return nil, err
		}
		newRaw = raw
		return &models.UserSession{
			UserID:           active.UserID,
			FamilyID:         active.FamilyID,
			RefreshTokenHash: utils.HashRefreshToken(raw),
			ExpiresAt:        time.Now().Add(utils.RefreshTokenTTL),
			Role:             active.Role,
			CenterID:         active.CenterID,
			Email:            active.Email,
			Username:         active.Username,
		}, nil
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrSessionNotFound),
			errors.Is(err, repository.ErrSessionExpired),
			errors.Is(err, repository.ErrSessionReuse):
			utils.Log.Warnf("[AUTH] Refresh rejected (%v)", err)
			return RefreshResult{}, apperrors.ErrUnauthorized
		default:
			return RefreshResult{}, err
		}
	}

	accessToken, err := utils.GenerateJWT(newSession.UserID, newSession.CenterID, newSession.Role, newSession.Email, newSession.Username)
	if err != nil {
		return RefreshResult{}, err
	}
	return RefreshResult{AccessToken: accessToken, RefreshToken: newRaw}, nil
}

// ──────────────────────────────────────────────
//  LOGOUT
// ──────────────────────────────────────────────

// Logout revokes the session presenting rawToken (D3). It is idempotent:
// unknown or already-revoked tokens yield no error, so the endpoint always
// answers 204 for an authenticated caller.
func (s *AuthAccountServiceImpl) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return apperrors.ErrBadRequest
	}
	return s.SessionRepo.RevokeByHash(ctx, utils.HashRefreshToken(rawToken))
}
