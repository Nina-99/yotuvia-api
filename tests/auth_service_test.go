package tests

import (
	"os"
	"testing"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthServiceTestDB(t *testing.T) *gorm.DB {
	os.Setenv("JWT_SECRET", "suoer-secret-key-for-the-test-units-of-yotuvia")

	utils.InitLogger(utils.LoggerConfig{
		Level:  "error",
		Format: "console",
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Falied to migrate database: %v", err)
	}

	err = db.AutoMigrate(
		&models.Client{},
		&models.Personnel{},
		&models.BeautyCenter{},
		&models.AuthAccount{},
		&models.UserSession{},
	)
	if err != nil {
		t.Fatalf("Falied to migrate database: %v", err)
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	client := models.Client{
		FirstName: "Juan",
		LastName:  "Perez",
		Email:     "juan@perez.com",
		Username:  "juanperez",
		Password:  string(hashedPassword),
	}
	db.Create(&client)

	return db
}

func TestAuthService_Login(t *testing.T) {
	db := setupAuthServiceTestDB(t)
	validate := validator.New()
	// Instanciamos los repositorios reales usando la DB en memoria
	authRepo := repository.NewAuthAccountRepositoryImpl(db)
	clientRepo := repository.NewClientRepositoryImpl(db)
	personnelRepo := repository.NewPersonnelRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	// Instanciamos el servicio unificado
	authService := service.NewAuthAccountServiceImpl(authRepo, clientRepo, personnelRepo, centerRepo,
		repository.NewPlatformAdminRepositoryImpl(db),
		repository.NewUserSessionRepositoryImpl(db), repository.NewAuthTokenRepositoryImpl(db),
		utils.NewFakeMailer(), nil, validate)
	t.Run("Classic Login Success (Client)", func(t *testing.T) {
		req := request.LoginRequest{
			Identifier: "juan@perez.com", // Buscamos por email
			Password:   "password123",
			UserType:   "client",
		}
		token, err := authService.Login(req)
		assert.NoError(t, err)
		assert.NotEmpty(t, token) // El token no debe estar vacío
	})
	t.Run("Classic Login Invalid Credentials", func(t *testing.T) {
		req := request.LoginRequest{
			Identifier: "juan@perez.com",
			Password:   "password_errada",
			UserType:   "client",
		}
		token, err := authService.Login(req)
		assert.Error(t, err)
		assert.Empty(t, token)
		assert.Equal(t, "invalid credentials", err.Error())
	})
}
