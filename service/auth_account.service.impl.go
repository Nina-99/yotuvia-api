package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
)

type AuthAccountServiceImpl struct {
	AuthRepo      repository.AuthAccountRepository
	ClientRepo    repository.ClientRepository
	PersonnelRepo repository.PersonnelRepository
	CenterRepo    repository.BeautyCenterRepository
	Validate      *validator.Validate
}

func NewAuthAccountServiceImpl(
	authRepo repository.AuthAccountRepository,
	clientRepo repository.ClientRepository,
	personnelRepo repository.PersonnelRepository,
	centerRepo repository.BeautyCenterRepository,
	validate *validator.Validate,
) AuthAccountService {
	return &AuthAccountServiceImpl{
		AuthRepo:      authRepo,
		ClientRepo:    clientRepo,
		PersonnelRepo: personnelRepo,
		CenterRepo:    centerRepo,
		Validate:      validate,
	}
}

// 1. LOGIN CLASIC (Email/Username + Password)
func (s *AuthAccountServiceImpl) Login(req request.LoginRequest) (string, error) {
	if err := s.Validate.Struct(req); err != nil {
		return "", err
	}

	var userID uint
	var centerID uint
	var email string
	var username string
	var passwordHash string

	switch req.UserType {
	case "client":
		var client *models.Client
		var err error
		if utils.IsEmail(req.Identifier) {
			client, err = s.ClientRepo.FindByEmail(req.Identifier)
		} else {
			client, err = s.ClientRepo.FindByUsername(req.Identifier)
		}
		if err != nil || client == nil {
			return "", errors.New("invalid credentials")
		}
		userID = client.ID
		centerID = 0
		email = client.Email
		username = client.Username
		passwordHash = client.Password

	case "personnel":
		personnel, err := s.PersonnelRepo.FindByEmail(req.Identifier)
		if err != nil || personnel == nil {
			return "", errors.New("invalid credentials")
		}
		userID = personnel.ID
		centerID = personnel.BeautyCenterID
		email = personnel.Email
		username = personnel.FirstName
		passwordHash = personnel.Password

	case "beauty_center":
		center, err := s.CenterRepo.FindByEmail(req.Identifier)
		if err != nil || center == nil {
			return "", errors.New("invalid credentials")
		}
		userID = center.ID
		centerID = center.ID
		email = center.Email
		username = center.Name
		passwordHash = center.Password
	}

	if passwordHash == "" {
		return "", errors.New("please log in using your social account or set a password first")
	}

	if !utils.VerifyPassword(req.Password, passwordHash) {
		return "", errors.New("invalid credentials")
	}

	utils.Log.Infof("[AUTH] Classic Login successful. UserType: %s, ID: %d", req.UserType, userID)
	return utils.GenerateJWT(userID, centerID, req.UserType, email, username)
}

// 2. SOCIAL LOGIN (Google/Facebook OAuth) con Account Linking
func (s *AuthAccountServiceImpl) SocialLogin(req request.SocialLoginRequest) (string, error) {
	if err := s.Validate.Struct(req); err != nil {
		return "", err
	}

	var providerID string
	var email string
	var firstName string
	var lastName string
	var picture string

	// 1. Validate the token
	if req.Provider == "google" {
		userInfo, err := utils.VerifyGoogleToken(req.AccessToken)
		if err != nil {
			return "", err
		}
		providerID = userInfo.ID
		email = userInfo.Email
		firstName = userInfo.GivenName
		lastName = userInfo.FamilyName
		picture = userInfo.Picture
	} else {
		userInfo, err := utils.VerifyFacebookToken(req.AccessToken)
		if err != nil {
			return "", err
		}
		providerID = userInfo.ID
		email = userInfo.Email
		firstName = userInfo.FirstName
		lastName = userInfo.LastName
		picture = userInfo.Picture.Data.URL
	}

	// 2. We search if the account exist in the DB
	account, err := s.AuthRepo.FindByProvider(req.Provider, providerID)
	if err != nil {
		return "", err
	}

	// 3. Case A: The account exist. Generate JWT and login of one
	if account != nil {
		utils.Log.Infof("[AUTH] Social Login successful. UserType: %s, Provider: %s", account.UserType, req.Provider)
		return s.generateTokenForUser(account)
	}

	// Case B: The Caccount not exist. Register automatic progresive
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
			return "", err
		}
		newAccount.ClientID = &client.ID

	case "personnel":
		personnel := models.Personnel{
			FirstName: firstName,
			LastName:  lastName,
			Email:     email,
			IsActive:  false, // Waiting for invitation
			PersonnelImage: []models.PersonnelImage{
				{
					ImageUrl: picture,
					IsCover:  true,
					Position: 1,
				},
			},
		}
		if err := s.PersonnelRepo.Create(&personnel); err != nil {
			return "", err
		}
		newAccount.PersonnelID = &personnel.ID

	case "beauty_center":
		center := models.BeautyCenter{
			Name:     firstName + " " + lastName,
			Email:    email,
			IsActive: false, // Flag IsOnBoard = false
			BeautyCenterImage: []models.BeautyCenterImage{
				{
					ImageUrl: picture,
					IsCover:  true,
					Position: 1,
				},
			},
		}
		if err := s.CenterRepo.Create(&center); err != nil {
			return "", err
		}
		newAccount.BeautyCenterID = &center.ID

	}
	if err := s.AuthRepo.Create(&newAccount); err != nil {
		return "", err
	}

	utils.Log.Infof("[AUTH] Created new social link and logged in. UserType: %s", req.UserType)
	return s.generateTokenForUser(&newAccount)
}

func (s *AuthAccountServiceImpl) generateTokenForUser(account *models.AuthAccount) (string, error) {
	var userID uint
	var centerID uint
	var email string
	var username string

	switch account.UserType {
	case "client":
		if account.ClientID != nil {
			userID = *account.ClientID
			centerID = 0
			client, _ := s.ClientRepo.FindById(userID)
			if client != nil {
				email = client.Email
				username = client.Username
			}
		}
	case "personnel":
		if account.PersonnelID != nil {
			userID = *account.PersonnelID
			personnel, _ := s.PersonnelRepo.FindByID(userID)
			if personnel != nil {
				email = personnel.Email
				username = personnel.FirstName
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
			}
		}
	}

	if userID == 0 {
		return "", errors.New("falied to identify user ID associated with this social account")
	}

	return utils.GenerateJWT(userID, centerID, account.UserType, email, username)
}

func (s *AuthAccountServiceImpl) Register(req request.RegisterRequest) (string, error) {
	if err := s.Validate.Struct(req); err != nil {
		return "", err
	}
	// 1. Dplicate control according to role and insertion in the DB
	var userID uint
	var centerID uint
	var email string
	var name string

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return "", err
	}
	switch req.UserType {
	case "client":
		// A. Verify duplicate email in Clients
		existingEmail, _ := s.ClientRepo.FindByEmail(req.Email)
		if existingEmail != nil {
			return "", errors.New("email already exists")
		}
		usernameVal := req.Username
		if usernameVal == "" {
			usernameVal = req.Email
		}
		// B. Verify duplicate username in Clients
		existingUsername, _ := s.ClientRepo.FindByUsername(usernameVal)
		if existingUsername != nil {
			return "", errors.New("username already exists")
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
			return "", err
		}
		userID = client.ID
		centerID = 0
		email = client.Email
		name = client.Username

	case "personnel":
		existingEmail, _ := s.PersonnelRepo.FindByEmail(req.Email)
		if existingEmail != nil {
			return "", errors.New("email already exists")
		}
		personnel := models.Personnel{
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Email:     req.Email,
			Phone:     req.Phone,
			Password:  hashedPassword,
			IsActive:  false, // Awaiting partnership/invitation
		}
		if err := s.PersonnelRepo.Create(&personnel); err != nil {
			return "", err
		}
		userID = personnel.ID
		centerID = personnel.BeautyCenterID
		email = personnel.Email
		name = personnel.FirstName

	case "beauty_center":
		existingEmail, _ := s.CenterRepo.FindByEmail(req.Email)
		if existingEmail != nil {
			return "", errors.New("email already exists")
		}
		center := models.BeautyCenter{
			Name:     req.FirstName + " " + req.LastName,
			Email:    req.Email,
			Phone:    req.Phone,
			Password: hashedPassword,
			IsActive: false, // Flag de Onboarding pendiente
		}
		if err := s.CenterRepo.Create(&center); err != nil {
			return "", err
		}
		userID = center.ID
		centerID = center.ID
		email = center.Email
		name = center.Name
	}
	utils.Log.Infof("[AUTH] New %s registered successfully. ID: %d, Email: %s", req.UserType, userID, email)
	return utils.GenerateJWT(userID, centerID, req.UserType, email, name)
}
