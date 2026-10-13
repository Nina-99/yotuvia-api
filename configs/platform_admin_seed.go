package configs

import (
	"errors"
	"os"
	"strings"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"gorm.io/gorm"
)

// SeedPlatformAdmin creates the single platform-level administrator from the
// environment. It exists because the account must NEVER be reachable through the
// public registration endpoints: the only way a super_admin comes to exist is
// this seed.
//
// Contract:
//   - env-driven: SUPER_ADMIN_EMAIL + SUPER_ADMIN_PASSWORD (SUPER_ADMIN_NAME is
//     optional and defaults to "Platform Admin"). Either credential missing ⇒
//     logged no-op, nothing is created.
//   - idempotent and non-destructive: an admin with that email is left exactly as
//     it is (the stored hash is NEVER overwritten), so a re-run — including one
//     with a rotated SUPER_ADMIN_PASSWORD — cannot silently change the live
//     credential. Rotating the password is an explicit operator action.
//   - the plaintext password is never logged; only the email is.
func SeedPlatformAdmin() {
	if DB == nil {
		return
	}

	email := strings.TrimSpace(os.Getenv("SUPER_ADMIN_EMAIL"))
	password := os.Getenv("SUPER_ADMIN_PASSWORD")
	if email == "" || password == "" {
		utils.Log.Infof("[SEED] Platform admin seed skipped: SUPER_ADMIN_EMAIL and SUPER_ADMIN_PASSWORD must both be set")
		return
	}

	var existing models.PlatformAdmin
	err := DB.Where("email = ?", email).First(&existing).Error
	if err == nil {
		utils.Log.Infof("[SEED] Platform admin %s already exists — password left untouched", email)
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		utils.Log.Errorf("[SEED] Platform admin lookup failed for %s: %v", email, err)
		return
	}

	name := strings.TrimSpace(os.Getenv("SUPER_ADMIN_NAME"))
	if name == "" {
		name = "Platform Admin"
	}

	hashed, err := utils.HashPassword(password)
	if err != nil {
		utils.Log.Errorf("[SEED] Platform admin password hashing failed: %v", err)
		return
	}

	admin := models.PlatformAdmin{
		Email:    email,
		Password: hashed,
		Name:     name,
		IsActive: true,
	}
	if err := DB.Create(&admin).Error; err != nil {
		utils.Log.Errorf("[SEED] Failed to create platform admin %s: %v", email, err)
		return
	}
	utils.Log.Infof("[SEED] Created platform admin %s (id %d)", email, admin.ID)
}
