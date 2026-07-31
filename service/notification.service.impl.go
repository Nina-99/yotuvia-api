package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
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

func (s *NotificationServiceImpl) GetByClientID(clientID uint) ([]models.Notification, error) {
	return s.NotificationRepo.FindByClientID(clientID)
}

func (s *NotificationServiceImpl) GetUnreadByClientID(clientID uint) ([]models.Notification, error) {
	return s.NotificationRepo.FindUnreadByClientID(clientID)
}

func (s *NotificationServiceImpl) MarkAsRead(notificationID uint) error {
	return s.NotificationRepo.MarkAsRead(notificationID)
}

func (s *NotificationServiceImpl) MarkAllAsRead(clientID uint) error {
	return s.NotificationRepo.MarkAllAsRead(clientID)
}

func (s *NotificationServiceImpl) CountUnread(clientID uint) (int64, error) {
	return s.NotificationRepo.CountUnread(clientID)
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
