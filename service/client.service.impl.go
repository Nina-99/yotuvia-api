package service

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type ClientServiceImpl struct {
	ClientRepository repository.ClientRepository
	Validate         *validator.Validate
}

func NewClientServiceImpl(clientRepository repository.ClientRepository, validate *validator.Validate) ClientService {
	return &ClientServiceImpl{
		ClientRepository: clientRepository,
		Validate:         validate,
	}
}

func (s *ClientServiceImpl) Update(id uint, actorClientID uint, req request.UpdateClientRequest) error {
	// Ownership at entry: a client may only update its own profile (:id == claim).
	if actorClientID != id {
		return apperrors.ErrForbidden
	}

	if err := s.Validate.Struct(req); err != nil {
		utils.Log.Warnf("Update failed: validation error: %v", err)
		return apperrors.ErrValidation
	}

	client, err := s.ClientRepository.FindById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		utils.Log.Errorf("Update failed: client ID %d not found: %v", id, err)
		return err
	}

	if req.FirstName != "" {
		client.FirstName = req.FirstName
	}
	if req.LastName != "" {
		client.LastName = req.LastName
	}
	if req.Phone != "" {
		client.Phone = req.Phone
	}
	if req.Username != "" && req.Username != client.Username {
		existingUsername, err := s.ClientRepository.FindByUsername(req.Username)
		if err != nil {
			utils.Log.Errorf("Update failed: error checking username %s: %v", req.Username, err)
			return err
		}
		if existingUsername != nil {
			utils.Log.Warnf("Update failed: username %s already exists", req.Username)
			return apperrors.ErrConflict
		}
		client.Username = req.Username
	}
	if req.Email != "" && req.Email != client.Email {
		existingEmail, err := s.ClientRepository.FindByEmail(req.Email)
		if err != nil {
			utils.Log.Errorf("Update failed: error checking email %s: %v", req.Email, err)
			return err
		}
		if existingEmail != nil {
			utils.Log.Warnf("Update failed: email %s already exists", req.Email)
			return apperrors.ErrConflict
		}
		client.Email = req.Email
	}
	if req.Password != "" {
		hash, err := utils.HashPassword(req.Password)
		if err != nil {
			utils.Log.Errorf("Update failed: failed to hash password: %v", err)
			return err
		}
		client.Password = hash
	}
	if req.AvatarURL != "" {
		client.AvatarURL = req.AvatarURL
	}
	// Only when explicitly provided: the mobile client edit never sends
	// is_active, and a defaulted false would deactivate the account.
	if req.IsActive != nil {
		client.IsActive = *req.IsActive
	}

	if err := s.ClientRepository.Update(client); err != nil {
		utils.Log.Errorf("Update failed: error saving client to database: %v", err)
		return err
	}
	utils.Log.Infof("Client with ID %d updated successfully", id)
	return nil
}

func (s *ClientServiceImpl) Delete(id uint, actorClientID uint) error {
	// Ownership at entry: a client may only delete its own profile.
	if actorClientID != id {
		return apperrors.ErrForbidden
	}

	user, err := s.ClientRepository.FindById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		return err
	}
	return s.ClientRepository.Delete(user)
}

func (s *ClientServiceImpl) FindAllAdmin(centerID uint, page, limit int) ([]response.AdminClientResponse, int64, error) {
	clients, metrics, total, err := s.ClientRepository.FindAllAdmin(page, limit, centerID)
	if err != nil {
		return nil, 0, err
	}

	res := make([]response.AdminClientResponse, 0, len(clients))
	for i := range clients {
		res = append(res, response.AdminClientResponse{
			ID:          clients[i].ID,
			FirstName:   clients[i].FirstName,
			LastName:    clients[i].LastName,
			Email:       clients[i].Email,
			Phone:       clients[i].Phone,
			AvatarURL:   clients[i].AvatarURL,
			IsActive:    clients[i].IsActive,
			VisitsCount: metrics[i].VisitsCount,
			TotalSpent:  metrics[i].TotalSpent,
			LastVisit:   metrics[i].LastVisit,
		})
	}
	return res, total, nil
}

func (s *ClientServiceImpl) FindEmail(Email string) (*models.Client, error) {
	client, err := s.ClientRepository.FindByEmail(Email)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, apperrors.ErrNotFound
	}
	return client, nil
}

func (s *ClientServiceImpl) FindByID(id uint, actorClientID uint) (*models.Client, error) {
	// Ownership at entry: a client may only read its own profile.
	if actorClientID != id {
		return nil, apperrors.ErrForbidden
	}

	client, err := s.ClientRepository.FindById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return client, nil
}

// maxAvatarBytes is the 5MB upload cap from the spec.
const maxAvatarBytes = 5 * 1024 * 1024

// sniffedImageExtensions maps a detected image/* MIME type to the extension the
// stored file gets. The extension ALWAYS comes from the sniff, never from the
// client-controlled filename (spoofable, may carry path separators).
var sniffedImageExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

func (s *ClientServiceImpl) UploadAvatar(clientID uint, file *multipart.FileHeader) (string, error) {
	if file.Size > maxAvatarBytes {
		return "", apperrors.ErrFileTooLarge
	}

	src, err := file.Open()
	if err != nil {
		utils.Log.Errorf("UploadAvatar failed: cannot open upload for client %d: %v", clientID, err)
		return "", err
	}
	defer src.Close()

	// Sniff the first 512 bytes for the real MIME type (mirrors the spec's
	// "detected MIME type" requirement and http.DetectContentType's window).
	head := make([]byte, 512)
	n := 0
	for n < len(head) {
		read, err := src.Read(head[n:])
		n += read
		if err != nil {
			break
		}
	}
	mime := http.DetectContentType(head[:n])
	if !strings.HasPrefix(mime, "image/") {
		return "", apperrors.ErrValidation
	}
	ext, ok := sniffedImageExtensions[mime]
	if !ok {
		utils.Log.Warnf("UploadAvatar rejected unsupported image type %q for client %d", mime, clientID)
		return "", apperrors.ErrValidation
	}

	uploadDir := filepath.Join("uploads", "clients", strconv.FormatUint(uint64(clientID), 10))
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		utils.Log.Errorf("UploadAvatar failed: cannot create upload dir: %v", err)
		return "", err
	}

	safeName := fmt.Sprintf("%d_%d%s", clientID, time.Now().UnixNano(), ext)
	filePath := filepath.Join(uploadDir, safeName)
	if err := utils.SaveUploadedFile(file, filePath); err != nil {
		utils.Log.Errorf("UploadAvatar failed: cannot save file for client %d: %v", clientID, err)
		return "", err
	}

	avatarURL := fmt.Sprintf("/uploads/clients/%d/%s", clientID, safeName)

	// Persist it HERE. Attaching the uploaded image to the client is the whole
	// point of this endpoint; leaving it to the caller meant the web did the
	// extra PUT and the mobile silently discarded the returned URL, so the
	// avatar was saved to disk and then vanished on the next reload.
	//
	// The value stored stays RELATIVE on purpose: it is portable across hosts,
	// and clients resolve it against their own asset origin on read.
	client, err := s.ClientRepository.FindById(clientID)
	if err != nil || client == nil {
		utils.Log.Errorf("UploadAvatar failed: cannot load client %d: %v", clientID, err)
		return "", err
	}
	client.AvatarURL = avatarURL
	if err := s.ClientRepository.Update(client); err != nil {
		utils.Log.Errorf("UploadAvatar failed: cannot persist avatar for client %d: %v", clientID, err)
		return "", err
	}

	return avatarURL, nil
}
