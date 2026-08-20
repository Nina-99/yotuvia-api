package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"gorm.io/gorm"
)

type NotificationServiceImpl struct {
	NotificationRepo repository.NotificationRepository
	PushDeviceRepo   repository.PushDeviceRepository
}

func NewNotificationServiceImpl(
	notificationRepo repository.NotificationRepository,
	pushDeviceRepo repository.PushDeviceRepository,
) NotificationService {
	return &NotificationServiceImpl{
		NotificationRepo: notificationRepo,
		PushDeviceRepo:   pushDeviceRepo,
	}
}

// ──────────────────────────────────────────────
//  IN-APP NOTIFICATIONS
// ──────────────────────────────────────────────

func (s *NotificationServiceImpl) Create(clientID uint, notifType, title, message, data string) error {
	notif := &models.Notification{
		ClientID: clientID,
		Type:     notifType,
		Title:    title,
		Message:  message,
		Data:     data,
	}
	return s.NotificationRepo.Create(notif)
}

func (s *NotificationServiceImpl) GetByClientID(clientID, actorClientID uint) ([]models.Notification, error) {
	// Owner-only: the path clientId must match the actor's clientID claim.
	if actorClientID != clientID {
		return nil, apperrors.ErrForbidden
	}
	return s.NotificationRepo.FindByClientID(clientID)
}

func (s *NotificationServiceImpl) GetUnreadByClientID(clientID, actorClientID uint) ([]models.Notification, error) {
	if actorClientID != clientID {
		return nil, apperrors.ErrForbidden
	}
	return s.NotificationRepo.FindUnreadByClientID(clientID)
}

func (s *NotificationServiceImpl) MarkAsRead(notificationID, actorClientID uint) error {
	notif, err := s.NotificationRepo.FindByID(notificationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		return err
	}
	// Owner-only: the notification must belong to the actor's client.
	if notif.ClientID != actorClientID {
		return apperrors.ErrForbidden
	}
	return s.NotificationRepo.MarkAsRead(notificationID)
}

func (s *NotificationServiceImpl) MarkAllAsRead(clientID, actorClientID uint) error {
	if actorClientID != clientID {
		return apperrors.ErrForbidden
	}
	return s.NotificationRepo.MarkAllAsRead(clientID)
}

func (s *NotificationServiceImpl) CountUnread(clientID, actorClientID uint) (int64, error) {
	if actorClientID != clientID {
		return 0, apperrors.ErrForbidden
	}
	return s.NotificationRepo.CountUnread(clientID)
}

// ──────────────────────────────────────────────
//  PERSONNEL-SCOPED NOTIFICATIONS (design D5)
// ──────────────────────────────────────────────

func (s *NotificationServiceImpl) CreateForPersonnel(personnelID uint, notifType, title, message, data string) error {
	notif := &models.Notification{
		ClientID:    0, // D5 sentinel: staff rows never belong to a real client
		PersonnelID: &personnelID,
		Type:        notifType,
		Title:       title,
		Message:     message,
		Data:        data,
	}
	return s.NotificationRepo.Create(notif)
}

func (s *NotificationServiceImpl) GetByPersonnelID(personnelID, actorPersonnelID uint) ([]models.Notification, error) {
	// Owner-only: the path personnelId must match the actor's claim.
	if actorPersonnelID != personnelID {
		return nil, apperrors.ErrForbidden
	}
	return s.NotificationRepo.FindByPersonnelID(personnelID)
}

func (s *NotificationServiceImpl) GetUnreadByPersonnelID(personnelID, actorPersonnelID uint) ([]models.Notification, error) {
	if actorPersonnelID != personnelID {
		return nil, apperrors.ErrForbidden
	}
	return s.NotificationRepo.FindUnreadByPersonnelID(personnelID)
}

func (s *NotificationServiceImpl) CountUnreadByPersonnelID(personnelID, actorPersonnelID uint) (int64, error) {
	if actorPersonnelID != personnelID {
		return 0, apperrors.ErrForbidden
	}
	return s.NotificationRepo.CountUnreadByPersonnelID(personnelID)
}

func (s *NotificationServiceImpl) MarkAsReadByPersonnel(notificationID, actorPersonnelID uint) error {
	notif, err := s.NotificationRepo.FindByID(notificationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		return err
	}
	// Nil-safe ownership: client rows have PersonnelID nil and can never be
	// claimed by staff, so a mismatch (nil or different value) is forbidden.
	if notif.PersonnelID == nil || *notif.PersonnelID != actorPersonnelID {
		return apperrors.ErrForbidden
	}
	return s.NotificationRepo.MarkAsRead(notificationID)
}

// ──────────────────────────────────────────────
//  CENTER-SCOPED NOTIFICATIONS (design D1/D3)
// ──────────────────────────────────────────────

func (s *NotificationServiceImpl) CreateForCenter(centerID uint, notifType, title, message, data string) error {
	notif := &models.Notification{
		ClientID:       0, // D1 sentinel: center rows never belong to a real client
		BeautyCenterID: &centerID,
		Type:           notifType,
		Title:          title,
		Message:        message,
		Data:           data,
	}
	return s.NotificationRepo.Create(notif)
}

func (s *NotificationServiceImpl) GetByCenterID(centerID, actorCenterID uint) ([]models.Notification, error) {
	// Owner-only: the path centerID must match the actor's claim.
	if actorCenterID != centerID {
		return nil, apperrors.ErrForbidden
	}
	return s.NotificationRepo.FindByCenterID(centerID)
}

func (s *NotificationServiceImpl) GetUnreadByCenterID(centerID, actorCenterID uint) ([]models.Notification, error) {
	if actorCenterID != centerID {
		return nil, apperrors.ErrForbidden
	}
	return s.NotificationRepo.FindUnreadByCenterID(centerID)
}

func (s *NotificationServiceImpl) CountUnreadByCenterID(centerID, actorCenterID uint) (int64, error) {
	if actorCenterID != centerID {
		return 0, apperrors.ErrForbidden
	}
	return s.NotificationRepo.CountUnreadByCenterID(centerID)
}

func (s *NotificationServiceImpl) MarkAsReadByCenter(notificationID, actorCenterID uint) error {
	notif, err := s.NotificationRepo.FindByID(notificationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		return err
	}
	// Nil-safe ownership: client/staff rows have BeautyCenterID nil and can
	// never be claimed by a center, so a mismatch (nil or different value) is
	// forbidden. Idempotent by construction: MarkAsRead on an already-read row
	// rewrites the same state.
	if notif.BeautyCenterID == nil || *notif.BeautyCenterID != actorCenterID {
		return apperrors.ErrForbidden
	}
	return s.NotificationRepo.MarkAsRead(notificationID)
}

// ──────────────────────────────────────────────
//  PUSH DEVICES
// ──────────────────────────────────────────────

func (s *NotificationServiceImpl) RegisterPushDevice(clientID uint, pushToken, platform string) error {
	device := &models.PushDevice{
		ClientID:  clientID,
		PushToken: pushToken,
		Platform:  platform,
	}
	return s.PushDeviceRepo.Register(device)
}

func (s *NotificationServiceImpl) UnregisterPushDevice(token string) error {
	return s.PushDeviceRepo.Unregister(token)
}

// ──────────────────────────────────────────────
//  SEND PUSH via Expo Push API
// ──────────────────────────────────────────────

type expoPushMessage struct {
	To       string          `json:"to"`
	Title    string          `json:"title"`
	Body     string          `json:"body"`
	Data     json.RawMessage `json:"data,omitempty"`
	Sound    string          `json:"sound,omitempty"`
	Priority string          `json:"priority,omitempty"`
}

func (s *NotificationServiceImpl) SendPushToClient(clientID uint, title, body, dataJSON string) error {
	devices, err := s.PushDeviceRepo.FindByClientID(clientID)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return nil // No devices registered, silently skip
	}

	// Expo Push API accepts up to 100 messages per request
	var messages []expoPushMessage
	for _, d := range devices {
		msg := expoPushMessage{
			To:       d.PushToken,
			Title:    title,
			Body:     body,
			Sound:    "default",
			Priority: "high",
		}
		if dataJSON != "" {
			msg.Data = json.RawMessage(dataJSON)
		}
		messages = append(messages, msg)
	}

	payload, err := json.Marshal(messages)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(
		"https://exp.host/--/api/v2/push/send",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("push send failed: %w", err)
	}
	defer resp.Body.Close()

	// Expo returns 200 OK even for partial failures — we log but don't fail the operation
	return nil
}
