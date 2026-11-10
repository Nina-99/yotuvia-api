package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
)

type AuthAccountServiceImpl struct {
	AuthRepo          repository.AuthAccountRepository
	ClientRepo        repository.ClientRepository
	PersonnelRepo     repository.PersonnelRepository
	CenterRepo        repository.BeautyCenterRepository
	PlatformAdminRepo repository.PlatformAdminRepository
	SessionRepo       repository.UserSessionRepository
	TokenRepo         repository.AuthTokenRepository
	Mailer            utils.Mailer
	TokenAccountSvc   TokenAccountService
	Validate          *validator.Validate
}

func NewAuthAccountServiceImpl(
	authRepo repository.AuthAccountRepository,
	clientRepo repository.ClientRepository,
	personnelRepo repository.PersonnelRepository,
	centerRepo repository.BeautyCenterRepository,
	platformAdminRepo repository.PlatformAdminRepository,
	sessionRepo repository.UserSessionRepository,
	tokenRepo repository.AuthTokenRepository,
	mailer utils.Mailer,
	tokenAccountSvc TokenAccountService,
	validate *validator.Validate,
) AuthAccountService {
	if mailer == nil {
		// A nil mailer must never panic a request; fall back to the log-only
		// transport (tests that don't exercise email still get a working flow).
		mailer = utils.NewDevMailer()
	}
	return &AuthAccountServiceImpl{
		AuthRepo:          authRepo,
		ClientRepo:        clientRepo,
		PersonnelRepo:     personnelRepo,
		CenterRepo:        centerRepo,
		PlatformAdminRepo: platformAdminRepo,
		SessionRepo:       sessionRepo,
		TokenRepo:         tokenRepo,
		Mailer:            mailer,
		TokenAccountSvc:   tokenAccountSvc,
		Validate:          validate,
	}
}

// DefaultMaxActiveSessions caps how many sessions one (account, role) pair may
// keep active at the same time. There is no usable device identity to key on
// (UserSession.DeviceInfo is never populated), so a "session" is one login and
// the cap is really "how many places may this account+role be signed in at
// once". Overridable with MAX_ACTIVE_SESSIONS.
const DefaultMaxActiveSessions = 2

// maxActiveSessions resolves the concurrent-session cap from
// MAX_ACTIVE_SESSIONS (plain os.Getenv, as in configs/scheduler.go). An unset,
// non-numeric, blank or sub-1 value falls back to DefaultMaxActiveSessions: a
// typo must never silently disable the cap.
func maxActiveSessions() int {
	raw := strings.TrimSpace(os.Getenv("MAX_ACTIVE_SESSIONS"))
	if raw == "" {
		return DefaultMaxActiveSessions
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 {
		utils.Log.Warnf("[AUTH] ignoring invalid MAX_ACTIVE_SESSIONS=%q; falling back to %d", raw, DefaultMaxActiveSessions)
		return DefaultMaxActiveSessions
	}
	return limit
}

// mintSession creates a new active session generation for the authenticated
// user and returns the raw refresh token to hand to the client exactly once,
// plus how many older sessions the cap forced out.
// Sessions key on the polymorphic JWT user_id claim (D1).
//
// The cap is enforced AFTER the insert (so the new session is part of the
// count): when the account+role now exceeds MAX_ACTIVE_SESSIONS, the oldest
// session(s) are revoked until exactly `limit` remain. The new login always
// succeeds — that is the point of option A: the user is never locked out of
// the device they are using right now.
func (s *AuthAccountServiceImpl) mintSession(userID, centerID uint, role, email, username string) (string, int, error) {
	raw, err := utils.GenerateRefreshToken()
	if err != nil {
		return "", 0, err
	}
	familyID, err := utils.GenerateFamilyID()
	if err != nil {
		return "", 0, err
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
		return "", 0, err
	}

	evicted, err := s.SessionRepo.EnforceActiveLimit(context.Background(), userID, role, maxActiveSessions(), time.Now())
	if err != nil {
		// The session exists and is valid, so the login MUST NOT fail here: a
		// failed enforcement degrades to "one session over the cap", never to
		// "the user cannot sign in". The client simply gets no warning.
		utils.Log.Errorf("[AUTH] session limit not enforced for %s #%d: %v", role, userID, err)
		return raw, 0, nil
	}
	return raw, evicted, nil
}

// termsVersion returns the server-authoritative Terms version stamped on every
// accepted registration. It is read from TERMS_VERSION (plain os.Getenv, as in
// configs/scheduler.go) and defaults to "1" when unset. A client-supplied
// version is never trusted.
func termsVersion() string {
	if v := os.Getenv("TERMS_VERSION"); v != "" {
		return v
	}
	return "1"
}

// recordTermsAcceptance persists the server-stamped Terms acceptance for a
// freshly registered user and returns the created account so the caller can key
// the verification token to it. AuthAccount is the account-audit record; a
// classic registration has no provider link, so it writes a "local" one. A
// failure is returned: an unrecorded acceptance must not masquerade as a
// successful registration.
func (s *AuthAccountServiceImpl) recordTermsAcceptance(userType, providerID string, clientID, personnelID, centerID *uint) (*models.AuthAccount, error) {
	now := time.Now()
	account := models.AuthAccount{
		Provider:        "local",
		ProviderID:      providerID,
		UserType:        userType,
		ClientID:        clientID,
		PersonnelID:     personnelID,
		BeautyCenterID:  centerID,
		TermsAcceptedAt: &now,
		TermsVersion:    termsVersion(),
	}
	if err := s.AuthRepo.Create(&account); err != nil {
		utils.Log.Errorf("[AUTH] Failed to record terms acceptance for %s: %v", userType, err)
		return nil, err
	}
	return &account, nil
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

	case "super_admin":
		// The platform-level principal lives in its own table and is resolved by
		// email only (it has no username and never a social link). It belongs to
		// NO centre, so centerID stays 0 and the JWT/middleware omit the claim.
		if s.PlatformAdminRepo == nil {
			return AuthResult{}, errors.New("invalid credentials")
		}
		admin, err := s.PlatformAdminRepo.FindByEmail(identifier)
		if err != nil || admin == nil || !admin.IsActive || admin.Password == "" {
			return AuthResult{}, errors.New("invalid credentials")
		}
		userID = admin.ID
		centerID = 0
		email = admin.Email
		username = admin.Name
		firstName = admin.Name
		lastName = ""
		passwordHash = admin.Password
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

	refreshToken, evicted, err := s.mintSession(userID, centerID, userType, email, username)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		Token:           token,
		RefreshToken:    refreshToken,
		UserID:          userID,
		Email:           email,
		FirstName:       firstName,
		LastName:        lastName,
		Role:            userType,
		SessionsEvicted: evicted,
	}, nil
}

func (s *AuthAccountServiceImpl) tryLoginAllTypes(identifier, password string) (AuthResult, error) {
	// super_admin is LAST on purpose: it can only ever match the admin's own
	// email, so every other role resolves exactly as before and the generic
	// "invalid credentials" outcome is unchanged.
	types := []string{"client", "beauty_center", "personnel", "super_admin"}

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

// generateUniqueReferralCode retries the random generator until it finds a code
// no center owns, so the unique index is a safety net, not the failure path.
func (s *AuthAccountServiceImpl) generateUniqueReferralCode() (string, error) {
	for i := 0; i < 10; i++ {
		code, err := utils.GenerateReferralCode()
		if err != nil {
			return "", err
		}
		existing, err := s.CenterRepo.FindByReferralCode(code)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return code, nil
		}
	}
	return "", errors.New("failed to generate a unique referral code")
}

func (s *AuthAccountServiceImpl) Register(req request.RegisterRequest) (AuthResult, error) {
	if err := s.Validate.Struct(req); err != nil {
		return AuthResult{}, fmt.Errorf("%w: %v", apperrors.ErrValidation, err)
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
	// verificationAccount is the local AuthAccount created for this
	// registration; the verification token is keyed to it.
	var verificationAccount *models.AuthAccount

	switch req.UserType {
	case "client":
		existingEmail, _ := s.ClientRepo.FindByEmail(req.Email)
		if existingEmail != nil {
			return AuthResult{}, apperrors.ErrEmailTaken
		}
		usernameVal := req.Username
		if usernameVal == "" {
			usernameVal = req.Email
		}
		existingUsername, _ := s.ClientRepo.FindByUsername(usernameVal)
		if existingUsername != nil {
			return AuthResult{}, apperrors.ErrUsernameTaken
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
		if verificationAccount, err = s.recordTermsAcceptance("client", client.Email, &client.ID, nil, nil); err != nil {
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
			return AuthResult{}, apperrors.ErrEmailTaken
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
		if verificationAccount, err = s.recordTermsAcceptance("personnel", personnel.Email, nil, &personnel.ID, nil); err != nil {
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
			return AuthResult{}, apperrors.ErrEmailTaken
		}
		// Generate a collision-safe unique referral code
		refCode, err := s.generateUniqueReferralCode()
		if err != nil {
			return AuthResult{}, err
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
		if verificationAccount, err = s.recordTermsAcceptance("beauty_center", center.Email, nil, nil, &center.ID); err != nil {
			return AuthResult{}, err
		}
		userID = center.ID
		centerID = center.ID
		email = center.Email
		username = center.Name
		firstName = center.Name
		lastName = ""

	default:
		return AuthResult{}, errors.New("invalid user type")
	}

	// Best-effort verification email: a mail failure must NOT fail
	// registration. The account is created and usable immediately (login is not
	// gated on verification); the user can always resend later.
	s.sendVerificationEmail(context.Background(), verificationAccount, req.UserType, userID, email, firstName)

	utils.Log.Infof("[AUTH] New %s registered successfully. ID: %d, Email: %s", req.UserType, userID, email)
	token, err := utils.GenerateJWT(userID, centerID, req.UserType, email, username)
	if err != nil {
		return AuthResult{}, err
	}

	refreshToken, evicted, err := s.mintSession(userID, centerID, req.UserType, email, username)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		Token:           token,
		RefreshToken:    refreshToken,
		UserID:          userID,
		Email:           email,
		FirstName:       firstName,
		LastName:        lastName,
		Role:            req.UserType,
		SessionsEvicted: evicted,
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
		refCode, err := s.generateUniqueReferralCode()
		if err != nil {
			return AuthResult{}, err
		}
		center := models.BeautyCenter{
			Name:         firstName + " " + lastName,
			Email:        email,
			IsActive:     false,
			ReferralCode: refCode,
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
		return fmt.Errorf("%w: %v", apperrors.ErrValidation, err)
	}

	// Check existing email
	existing, _ := s.PersonnelRepo.FindByEmail(req.Email)
	if existing != nil {
		return apperrors.ErrEmailTaken
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

	refreshToken, evicted, err := s.mintSession(userID, centerID, account.UserType, email, username)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		Token:           token,
		RefreshToken:    refreshToken,
		UserID:          userID,
		Email:           email,
		FirstName:       firstName,
		LastName:        lastName,
		Role:            account.UserType,
		SessionsEvicted: evicted,
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

// LogoutAll revokes every session of the acting account across every family
// and device. The actor is identified by the JWT claims (user_id + role), never
// by the request body. It also revokes the calling session: "sign out
// everywhere" is least surprising when it means everywhere, and the caller is
// expected to clear its local tokens on the 200 response. role is required
// because user_id is polymorphic (client #5 and center #5 share the numeric id),
// so revocation must not leak to another account type.
func (s *AuthAccountServiceImpl) LogoutAll(ctx context.Context, userID uint, role string) error {
	if userID == 0 || role == "" {
		// The JWT middleware guarantees both; a missing claim is not a valid
		// identity and must never fall through to a broad update.
		return apperrors.ErrUnauthorized
	}
	return s.SessionRepo.RevokeAllForUser(ctx, userID, role)
}

// ──────────────────────────────────────────────
//  EMAIL VERIFICATION
// ──────────────────────────────────────────────

// VerifyEmail redeems a verification token and stamps EmailVerifiedAt on the
// owning AuthAccount. Every invalid outcome — unknown, wrong purpose, already
// used, expired — maps to ErrInvalidToken (400) with one uniform message.
func (s *AuthAccountServiceImpl) VerifyEmail(ctx context.Context, rawToken string) error {
	token, err := s.consumeToken(ctx, rawToken, models.TokenPurposeEmailVerification)
	if err != nil {
		return err
	}
	if token.AccountID == nil {
		utils.Log.Errorf("[AUTH] verification token %d has no account", token.ID)
		return apperrors.ErrInvalidToken
	}
	account, err := s.AuthRepo.FindById(*token.AccountID)
	if err != nil || account == nil {
		utils.Log.Errorf("[AUTH] verification token %d references missing account %d: %v", token.ID, *token.AccountID, err)
		return apperrors.ErrInvalidToken
	}
	now := time.Now()
	account.EmailVerifiedAt = &now
	if err := s.AuthRepo.Update(account); err != nil {
		return err
	}
	utils.Log.Infof("[AUTH] email verified for %s account %d", account.UserType, account.ID)
	return nil
}

// ResendVerification issues and sends a new verification link when an
// unverified account exists for the email. It NEVER reveals whether the account
// exists (nor whether it is already verified): the returned error is nil for
// every well-formed address, and mail failures are logged, not surfaced.
func (s *AuthAccountServiceImpl) ResendVerification(ctx context.Context, req request.ResendVerificationRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return fmt.Errorf("%w: %v", apperrors.ErrValidation, err)
	}

	userType, userID, firstName, found := s.resolveUserByEmail(req.Email)
	if !found {
		utils.Log.Infof("[AUTH] resend-verification requested for unknown email; answering 200")
		return nil
	}
	account, err := s.AuthRepo.FindByLocalUser(userType, userID)
	if err != nil {
		utils.Log.Errorf("[AUTH] resend-verification account lookup failed: %v", err)
		return nil
	}
	// Already-verified: nothing to send, still silent.
	if account == nil || account.EmailVerifiedAt != nil {
		return nil
	}
	if err := s.issueAndSendVerification(ctx, account, userType, userID, req.Email, firstName); err != nil {
		utils.Log.Errorf("[AUTH] resend-verification email failed for %s: %v", req.Email, err)
	}
	return nil
}

// ──────────────────────────────────────────────
//  PASSWORD RECOVERY
// ──────────────────────────────────────────────

// ForgotPassword sends a reset link when the email belongs to an account and is
// otherwise silent (no enumeration). Mail failures are logged, never surfaced.
func (s *AuthAccountServiceImpl) ForgotPassword(ctx context.Context, req request.ForgotPasswordRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return fmt.Errorf("%w: %v", apperrors.ErrValidation, err)
	}

	userType, userID, firstName, found := s.resolveUserByEmail(req.Email)
	if !found {
		utils.Log.Infof("[AUTH] forgot-password requested for unknown email; answering 200")
		return nil
	}
	if err := s.issueAndSendPasswordReset(ctx, userType, userID, req.Email, firstName); err != nil {
		utils.Log.Errorf("[AUTH] forgot-password email failed for %s: %v", req.Email, err)
	}
	return nil
}

// ResetPassword redeems a single-use reset token, sets the new password and
// revokes EVERY session of that account across all devices. Order matters: the
// token is consumed first (so a replay after a successful reset is rejected),
// then the password is written, then sessions are killed. If a step after
// consumption fails, the token is spent and the user must request a new link —
// the safe direction for a credential-reset flow.
func (s *AuthAccountServiceImpl) ResetPassword(ctx context.Context, req request.ResetPasswordRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return fmt.Errorf("%w: %v", apperrors.ErrValidation, err)
	}

	token, err := s.consumeToken(ctx, req.Token, models.TokenPurposePasswordReset)
	if err != nil {
		return err
	}

	hashed, err := utils.HashPassword(req.Password)
	if err != nil {
		return err
	}
	if err := s.updatePassword(token.UserType, token.UserID, hashed); err != nil {
		return err
	}
	if err := s.SessionRepo.RevokeAllForUser(ctx, token.UserID, token.UserType); err != nil {
		return err
	}
	utils.Log.Infof("[AUTH] password reset for %s #%d; all sessions revoked", token.UserType, token.UserID)
	return nil
}

// ──────────────────────────────────────────────
//  EMAIL HELPERS
// ──────────────────────────────────────────────

// consumeToken validates a raw token against purpose/single-use/expiry and
// claims it atomically. A nil, wrong-purpose, used, expired or already-claimed
// token is ErrInvalidToken.
func (s *AuthAccountServiceImpl) consumeToken(ctx context.Context, rawToken, purpose string) (*models.AuthToken, error) {
	if rawToken == "" {
		return nil, apperrors.ErrInvalidToken
	}
	token, err := s.TokenRepo.FindByHash(ctx, utils.HashAuthToken(rawToken))
	if err != nil {
		return nil, err
	}
	if token == nil || token.Purpose != purpose || token.IsUsed() || token.IsExpired() {
		return nil, apperrors.ErrInvalidToken
	}
	claimed, err := s.TokenRepo.Consume(ctx, token.ID)
	if err != nil {
		return nil, err
	}
	if !claimed {
		// A concurrent request consumed it between read and CAS.
		return nil, apperrors.ErrInvalidToken
	}
	return token, nil
}

// sendVerificationEmail is the best-effort wrapper used by Register: any
// failure is logged and swallowed so registration still succeeds.
func (s *AuthAccountServiceImpl) sendVerificationEmail(ctx context.Context, account *models.AuthAccount, userType string, userID uint, email, firstName string) {
	if account == nil {
		return
	}
	if err := s.issueAndSendVerification(ctx, account, userType, userID, email, firstName); err != nil {
		utils.Log.Errorf("[AUTH] verification email not sent for %s (registration still succeeded): %v", email, err)
	}
}

// issueAndSendVerification retires any outstanding verification token, mints a
// new one, stores its hash and sends the Spanish email.
func (s *AuthAccountServiceImpl) issueAndSendVerification(ctx context.Context, account *models.AuthAccount, userType string, userID uint, email, firstName string) error {
	if err := s.TokenRepo.InvalidateOutstanding(ctx, userType, userID, models.TokenPurposeEmailVerification); err != nil {
		return err
	}
	raw, err := utils.GenerateAuthToken()
	if err != nil {
		return err
	}
	accountID := account.ID
	token := &models.AuthToken{
		UserType:  userType,
		UserID:    userID,
		AccountID: &accountID,
		Purpose:   models.TokenPurposeEmailVerification,
		TokenHash: utils.HashAuthToken(raw),
		ExpiresAt: time.Now().Add(utils.EmailVerificationTTL),
	}
	if err := s.TokenRepo.Create(ctx, token); err != nil {
		return err
	}
	content := utils.VerificationEmail(firstName, utils.VerificationLink(raw))
	return s.Mailer.Send(ctx, utils.Message{
		To:      email,
		Subject: content.Subject,
		Text:    content.Text,
		HTML:    content.HTML,
	})
}

// issueAndSendPasswordReset retires any outstanding reset token, mints a new
// one and sends the Spanish email.
func (s *AuthAccountServiceImpl) issueAndSendPasswordReset(ctx context.Context, userType string, userID uint, email, firstName string) error {
	var accountID *uint
	if account, err := s.AuthRepo.FindByLocalUser(userType, userID); err == nil && account != nil {
		accountID = &account.ID
	}
	if err := s.TokenRepo.InvalidateOutstanding(ctx, userType, userID, models.TokenPurposePasswordReset); err != nil {
		return err
	}
	raw, err := utils.GenerateAuthToken()
	if err != nil {
		return err
	}
	token := &models.AuthToken{
		UserType:  userType,
		UserID:    userID,
		AccountID: accountID,
		Purpose:   models.TokenPurposePasswordReset,
		TokenHash: utils.HashAuthToken(raw),
		ExpiresAt: time.Now().Add(utils.PasswordResetTTL),
	}
	if err := s.TokenRepo.Create(ctx, token); err != nil {
		return err
	}
	content := utils.PasswordResetEmail(firstName, utils.PasswordResetLink(raw))
	return s.Mailer.Send(ctx, utils.Message{
		To:      email,
		Subject: content.Subject,
		Text:    content.Text,
		HTML:    content.HTML,
	})
}

// resolveUserByEmail finds the account holder behind an email across the three
// user types. It returns found=false when nothing matches; callers must never
// turn that into a different HTTP response (enumeration).
func (s *AuthAccountServiceImpl) resolveUserByEmail(email string) (userType string, userID uint, firstName string, found bool) {
	if client, _ := s.ClientRepo.FindByEmail(email); client != nil {
		return "client", client.ID, client.FirstName, true
	}
	if personnel, _ := s.PersonnelRepo.FindByEmail(email); personnel != nil {
		return "personnel", personnel.ID, personnel.FirstName, true
	}
	if center, _ := s.CenterRepo.FindByEmail(email); center != nil {
		return "beauty_center", center.ID, center.Name, true
	}
	return "", 0, "", false
}

// updatePassword writes a new bcrypt hash onto the right domain entity.
func (s *AuthAccountServiceImpl) updatePassword(userType string, userID uint, hashed string) error {
	switch userType {
	case "client":
		client, err := s.ClientRepo.FindById(userID)
		if err != nil || client == nil {
			return errors.New("account no longer exists")
		}
		client.Password = hashed
		return s.ClientRepo.Update(client)
	case "personnel":
		personnel, err := s.PersonnelRepo.FindByID(userID)
		if err != nil || personnel == nil {
			return errors.New("account no longer exists")
		}
		personnel.Password = hashed
		return s.PersonnelRepo.Update(personnel)
	case "beauty_center":
		center, err := s.CenterRepo.FindByID(userID)
		if err != nil || center == nil {
			return errors.New("account no longer exists")
		}
		center.Password = hashed
		return s.CenterRepo.Update(center)
	default:
		return apperrors.ErrInvalidToken
	}
}
