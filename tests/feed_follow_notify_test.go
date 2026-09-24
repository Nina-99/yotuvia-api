package tests

import (
	"errors"
	"testing"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  New-video fan-out to the professional's followers
// ──────────────────────────────────────────────

// feedSvcWithNotif builds the real FeedPostService over the sqlite DB with the
// supplied notification service, so the fan-out can be driven (and sabotaged).
func feedSvcWithNotif(db *gorm.DB, notif service.NotificationService) service.FeedPostService {
	return service.NewFeedPostServiceImpl(
		repository.NewFeedPostRepositoryImpl(db),
		repository.NewBeautyCenterRepositoryImpl(db),
		repository.NewPersonnelRepositoryImpl(db),
		repository.NewServiceRepositoryImpl(db),
		repository.NewPostLikeRepositoryImpl(db),
		repository.NewSavedVideoRepositoryImpl(db),
		repository.NewVideoCommentRepositoryImpl(db),
		repository.NewPersonnelFollowRepositoryImpl(db),
		notif,
		validator.New(),
	)
}

// failingNotificationService embeds the interface so only the two fan-out
// methods need a definition; any other call would panic, which is desirable.
type failingNotificationService struct{ service.NotificationService }

func (failingNotificationService) Create(uint, string, string, string, string) error {
	return errors.New("notification backend down")
}

func (failingNotificationService) SendPushToClient(uint, string, string, string) error {
	return errors.New("push backend down")
}

func seedFollowNotifyFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedDashboardCenter(t, db, 1, "Notify Center", "notify@test.com", true)
	seedDashboardPersonnel(t, db, 7, 1, true)
	seedClient(t, db, 5, "f1", "f1@test.com")
	seedClient(t, db, 6, "f2", "f2@test.com")
	require.NoError(t, db.Create(&models.PersonnelFollow{ClientID: 5, PersonnelID: 7}).Error)
	require.NoError(t, db.Create(&models.PersonnelFollow{ClientID: 6, PersonnelID: 7}).Error)
}

// TestFeedFollow_NewVideoNotifiesEachFollowerOnce — publishing a video for a
// followed professional creates exactly one "new_video" notification per
// follower (and the post is authored correctly).
func TestFeedFollow_NewVideoNotifiesEachFollowerOnce(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedFollowNotifyFixtures(t, db)

	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	svc := feedSvcWithNotif(db, notifSvc)

	res, err := svc.CreateFromUpload(
		service.FeedActor{Role: "personnel", UserID: 7},
		request.CreateFeedPostRequest{Caption: "nuevo video"},
		fakeVideoFile(t, "notify.mp4"),
	)
	require.NoError(t, err)
	require.NotNil(t, res.PersonnelID)
	assert.Equal(t, uint(7), *res.PersonnelID)

	var rows []models.Notification
	require.NoError(t, db.Where("type = ?", "new_video").Find(&rows).Error)
	require.Len(t, rows, 2, "one notification per follower")

	perClient := map[uint]int{}
	for _, n := range rows {
		perClient[n.ClientID]++
		assert.Contains(t, n.Data, "new_video")
	}
	assert.Equal(t, 1, perClient[5], "follower 5 notified exactly once")
	assert.Equal(t, 1, perClient[6], "follower 6 notified exactly once")
}

// TestFeedFollow_NotificationFailureDoesNotFailCreate — a broken notification
// backend must never fail (or roll back) the post creation.
func TestFeedFollow_NotificationFailureDoesNotFailCreate(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedFollowNotifyFixtures(t, db)

	svc := feedSvcWithNotif(db, failingNotificationService{})

	res, err := svc.CreateFromUpload(
		service.FeedActor{Role: "personnel", UserID: 7},
		request.CreateFeedPostRequest{Caption: "still works"},
		fakeVideoFile(t, "resilient.mp4"),
	)
	require.NoError(t, err, "notification failure must not fail post creation")
	assert.NotZero(t, res.ID)

	var count int64
	require.NoError(t, db.Model(&models.FeedPost{}).Where("personnel_id = ?", 7).Count(&count).Error)
	assert.Equal(t, int64(1), count, "post must be persisted despite notification failure")
}
