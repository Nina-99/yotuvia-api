package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
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
	if err := s.NotificationRepo.Create(notif); err != nil {
		return err
	}
	// Same rule as the staff/centre paths: EVERY notification pushes, so the
	// recipient sees it with the app CLOSED, not only when it is open. Callers
	// must therefore NOT push separately, or the device gets it twice.
	_ = s.SendPushToOwner("client", clientID, title, message, data)
	return nil
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
	if err := s.NotificationRepo.Create(notif); err != nil {
		return err
	}
	// Best-effort push for EVERY staff notification: a transport failure must
	// never fail the write that produced the in-app row.
	_ = s.SendPushToOwner("personnel", personnelID, title, message, data)
	return nil
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
	if err := s.NotificationRepo.Create(notif); err != nil {
		return err
	}
	// Best-effort push to the CENTRE's devices, same rule as the staff path.
	_ = s.SendPushToOwner("beauty_center", centerID, title, message, data)
	return nil
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

// platformWeb marks a browser Push API subscription (as opposed to an Expo
// token). Kept as a constant so the send path can branch on ONE spelling.
const platformWeb = "web"

// vapidSettings reads the Web Push VAPID configuration from the environment.
// All-empty means Web Push is simply NOT configured: registration of a web
// device then fails closed at the service layer and the send path skips web
// silently, so a deployment without the keys keeps working exactly as before.
func vapidSettings() (public, private, subject string) {
	return os.Getenv("VAPID_PUBLIC_KEY"), os.Getenv("VAPID_PRIVATE_KEY"), os.Getenv("VAPID_SUBJECT")
}

func (s *NotificationServiceImpl) VapidPublicKey() string {
	public, _, _ := vapidSettings()
	return public
}

// RegisterPushDevice stores ONE device for one (role, owner). The role is what
// makes the table polymorphic: the same endpoint can belong to a client, a
// professional or a centre without the numeric ids ever colliding.
func (s *NotificationServiceImpl) RegisterPushDevice(role string, actorID uint, pushToken, platform, p256dh, auth string) error {
	if role == "" {
		role = "client"
	}
	device := &models.PushDevice{
		ClientID:  actorID,
		OwnerRole: role,
		PushToken: pushToken,
		Platform:  platform,
	}
	if platform == platformWeb {
		// A web subscription without its encryption keys can never be delivered,
		// so refuse it instead of storing a dead device.
		public, private, _ := vapidSettings()
		if public == "" || private == "" {
			return apperrors.ErrBadRequest
		}
		if p256dh == "" || auth == "" {
			return apperrors.ErrBadRequest
		}
		device.P256dh = &p256dh
		device.Auth = &auth
	}
	return s.PushDeviceRepo.Register(device)
}

func (s *NotificationServiceImpl) UnregisterPushDevice(token string, role string, actorID uint) error {
	return s.PushDeviceRepo.Unregister(token, role, actorID)
}

// ──────────────────────────────────────────────
//  SEND PUSH — native (Expo) + web (Web Push)
// ──────────────────────────────────────────────

type expoPushMessage struct {
	To       string          `json:"to"`
	Title    string          `json:"title"`
	Body     string          `json:"body"`
	Data     json.RawMessage `json:"data,omitempty"`
	Sound    string          `json:"sound,omitempty"`
	Priority string          `json:"priority,omitempty"`
	// ChannelID targets the Android notification channel. Sending it explicitly
	// guarantees a CLOSED Android app shows the alert on our own channel (with
	// its sound/importance) instead of a bare FCM fallback channel.
	ChannelID string `json:"channelId,omitempty"`
}

// webPushMessage is the JSON the service worker receives.
type webPushMessage struct {
	Title string          `json:"title"`
	Body  string          `json:"body"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// SendPushToClient delivers to every device the CLIENT registered.
func (s *NotificationServiceImpl) SendPushToClient(clientID uint, title, body, dataJSON string) error {
	return s.SendPushToOwner("client", clientID, title, body, dataJSON)
}

// SendPushToOwner delivers to every device one (role, owner) registered. Staff
// and owners reach this path; clients use the SendPushToClient wrapper so the
// existing call sites stay untouched.
func (s *NotificationServiceImpl) SendPushToOwner(role string, ownerID uint, title, body, dataJSON string) error {
	if role == "" {
		role = "client"
	}
	devices, err := s.PushDeviceRepo.FindByOwner(role, ownerID)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return nil // No devices registered, silently skip
	}

	native := make([]models.PushDevice, 0, len(devices))
	var web []models.PushDevice
	for _, d := range devices {
		if d.Platform == platformWeb {
			web = append(web, d)
		} else {
			native = append(native, d)
		}
	}

	if err := s.sendExpoPush(native, title, body, dataJSON); err != nil {
		return err
	}
	s.sendWebPush(web, title, body, dataJSON)
	return nil
}

// sendExpoPush is the original Expo delivery, unchanged.
func (s *NotificationServiceImpl) sendExpoPush(devices []models.PushDevice, title, body, dataJSON string) error {
	if len(devices) == 0 {
		return nil
	}
	// Expo Push API accepts up to 100 messages per request
	var messages []expoPushMessage
	for _, d := range devices {
		msg := expoPushMessage{
			To:        d.PushToken,
			Title:     title,
			Body:      body,
			Sound:     "default",
			Priority:  "high",
			ChannelID: "default",
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

// sendWebPush encrypts and delivers the payload to each web subscription
// (RFC 8291). A 404/410 from the push service means the subscription is gone,
// so the row is pruned. Every failure is logged, never returned.
func (s *NotificationServiceImpl) sendWebPush(devices []models.PushDevice, title, body, dataJSON string) {
	if len(devices) == 0 {
		return
	}
	public, private, subject := vapidSettings()
	if public == "" || private == "" {
		utils.Log.Warn("[PUSH] web devices registered but VAPID keys are not configured; skipping web push")
		return
	}
	if subject == "" {
		// The VAPID contact is mandatory in the JWT; a mailto: is the convention.
		subject = "mailto:no-reply@yotuvia.app"
	}

	payload := webPushMessage{Title: title, Body: body}
	if dataJSON != "" {
		payload.Data = json.RawMessage(dataJSON)
	}
	message, err := json.Marshal(payload)
	if err != nil {
		utils.Log.Errorf("[PUSH] web payload marshal failed: %v", err)
		return
	}

	for _, d := range devices {
		if d.P256dh == nil || d.Auth == nil {
			continue
		}
		subscription := &webpush.Subscription{
			Endpoint: d.PushToken,
			Keys:     webpush.Keys{P256dh: *d.P256dh, Auth: *d.Auth},
		}
		resp, err := webpush.SendNotification(message, subscription, &webpush.Options{
			Subscriber:      subject,
			TTL:             60,
			VAPIDPublicKey:  public,
			VAPIDPrivateKey: private,
		})
		if err != nil {
			utils.Log.Errorf("[PUSH] web push send failed: %v", err)
			continue
		}
		// 404/410 = the subscription no longer exists; drop it so the next send
		// does not keep retrying a dead endpoint.
		if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
			if err := s.PushDeviceRepo.Unregister(d.PushToken, d.OwnerRole, d.ClientID); err != nil {
				utils.Log.Errorf("[PUSH] pruning stale web subscription failed: %v", err)
			}
		}
		resp.Body.Close()
	}
}
