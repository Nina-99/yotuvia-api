package tests

import (
	"bytes"
	"mime/multipart"
	"os"
	"testing"

	"github.com/Nina-99/yottu-api/configs"
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
//  feat-video-feed-unification — Unit 1 (backend)
// ──────────────────────────────────────────────

func newFeedSUT(t *testing.T) (service.FeedPostService, *gorm.DB) {
	t.Helper()
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedDashboardCenter(t, db, 3, "Feed Center", "feed@test.com", true)
	seedDashboardPersonnel(t, db, 7, 3, true) // staff of center 3
	seedDashboardService(t, db, 12, 3, "Balayage", true)

	svc := service.NewFeedPostServiceImpl(
		repository.NewFeedPostRepositoryImpl(db),
		repository.NewBeautyCenterRepositoryImpl(db),
		repository.NewPersonnelRepositoryImpl(db),
		repository.NewServiceRepositoryImpl(db),
		repository.NewPostLikeRepositoryImpl(db),
		repository.NewSavedVideoRepositoryImpl(db),
		repository.NewVideoCommentRepositoryImpl(db),
		validator.New(),
	)
	return svc, db
}

// fakeVideoFile builds a multipart.FileHeader from an in-memory "mp4".
func fakeVideoFile(t *testing.T, name string) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("video", name)
	assert.NoError(t, err)
	_, err = part.Write([]byte("fake-video-bytes"))
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())

	reader := multipart.NewReader(bytes.NewReader(body.Bytes()), writer.Boundary())
	form, err := reader.ReadForm(1 << 20)
	assert.NoError(t, err)
	files := form.File["video"]
	if len(files) == 0 {
		t.Fatal("no parsed file header")
	}
	return files[0]
}

var staffActor = service.FeedActor{Role: "personnel", UserID: 7}
var centerActor = service.FeedActor{Role: "beauty_center", CenterID: 3}
var foreignCenterActor = service.FeedActor{Role: "beauty_center", CenterID: 4}

// TestFeed_StaffUploadForcesOwnIdentity covers D2: staff posts are forced to
// their own personnel id and derive the center from their record.
func TestFeed_StaffUploadForcesOwnIdentity(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, _ := newFeedSUT(t)

	file := fakeVideoFile(t, "work.mp4")
	req := request.CreateFeedPostRequest{
		Caption:     "Balayage paso a paso",
		ServiceID:   12,
		PersonnelID: 999, // spoof attempt — must be ignored/forced to 7
	}

	res, err := svc.CreateFromUpload(staffActor, req, file)
	assert.NoError(t, err)
	require.NotNil(t, res.PersonnelID)
	assert.Equal(t, uint(7), *res.PersonnelID, "personnel_id forced from claims")
	assert.Equal(t, uint(3), res.BeautyCenterID)
	assert.True(t, res.IsActive)
	assert.False(t, res.IsPromo)

	os.Remove("uploads/videos/" + file.Filename)
}

// TestFeed_CenterCannotCreditForeignStaff covers D2 validation.
func TestFeed_CenterCannotCreditForeignStaff(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, _ := newFeedSUT(t)

	// Foreign staff 9 does not exist in this DB -> credit rejected.
	file := fakeVideoFile(t, "promo.mp4")
	req := request.CreateFeedPostRequest{
		Caption:     "Promo",
		PersonnelID: 9,
		IsPromo:     true,
	}
	_, err := svc.CreateFromUpload(foreignCenterActor, req, file)
	assert.Error(t, err, "foreign staff credit must be rejected")

	os.Remove("uploads/videos/" + file.Filename)
}

// TestFeed_AuthorOnlyDelete enforces D3 deletion semantics.
func TestFeed_AuthorOnlyDelete(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, _ := newFeedSUT(t)

	file := fakeVideoFile(t, "mine.mp4")
	res, err := svc.CreateFromUpload(centerActor, request.CreateFeedPostRequest{
		Caption: "Trabajo del centro",
	}, file)
	assert.NoError(t, err)

	// Staff cannot delete a center-authored post...
	err = svc.Delete(service.FeedActor{Role: "personnel", UserID: 7}, res.ID)
	assert.Error(t, err)

	// ...foreign center cannot either...
	err = svc.Delete(foreignCenterActor, res.ID)
	assert.Error(t, err)

	// ...but its author can.
	assert.NoError(t, svc.Delete(centerActor, res.ID))

	os.Remove("uploads/videos/" + file.Filename)
}

// TestFeed_SuspendMatrix covers the full moderation lifecycle (D3): suspend
// hides from public feed + flags author view with note; re-enable restores;
// center-authored posts cannot be suspended.
func TestFeed_SuspendMatrix(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	svc, _ := newFeedSUT(t)

	file := fakeVideoFile(t, "staff-work.mp4")
	staffPost, err := svc.CreateFromUpload(staffActor, request.CreateFeedPostRequest{
		Caption:   "Mi trabajo",
		ServiceID: 12,
	}, file)
	assert.NoError(t, err)

	// Public feed sees it before suspension.
	public, _, err := svc.FindAll(1, 10, nil)
	assert.NoError(t, err)
	assert.Len(t, public, 1)

	// Suspend with note.
	assert.NoError(t, svc.SetSuspended(3, staffPost.ID, true, "Fuera de línea"))

	public, _, _ = svc.FindAll(1, 10, nil)
	assert.Len(t, public, 0, "suspended hidden from public feed")

	mine, err := svc.FindMine(staffActor)
	assert.NoError(t, err)
	if assert.Len(t, mine, 1) {
		assert.False(t, mine[0].IsActive)
		assert.Equal(t, "Fuera de línea", mine[0].ModerationNote)
	}

	// Re-enable clears note and restores visibility.
	assert.NoError(t, svc.SetSuspended(3, staffPost.ID, false, ""))
	public, _, _ = svc.FindAll(1, 10, nil)
	assert.Len(t, public, 1)
	mine, _ = svc.FindMine(staffActor)
	assert.Empty(t, mine[0].ModerationNote)

	// Center-authored posts cannot be suspended through this action.
	centerPost, err := svc.CreateFromUpload(centerActor, request.CreateFeedPostRequest{
		Caption: "Promo del centro", IsPromo: true,
	}, fakeVideoFile(t, "promo.mp4"))
	assert.NoError(t, err)
	err = svc.SetSuspended(3, centerPost.ID, true, "x")
	assert.Error(t, err, "center-authored posts cannot be suspended")

	os.Remove("uploads/videos/" + file.Filename)
	os.Remove("uploads/videos/promo.mp4")
}

// TestBackfill_LegacyVideos covers T4: legacy rows migrate into feed_posts
// preserving views, invalid rows are skipped, and the legacy table is dropped.
func TestBackfill_LegacyVideos(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	configs.DB = db
	defer func() { configs.DB = nil }()

	// Recreate a legacy-shaped table with data (model was removed).
	assert.NoError(t, db.Exec(`CREATE TABLE videos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		beauty_center_id INTEGER NOT NULL,
		title TEXT,
		description TEXT,
		file_path TEXT,
		thumbnail_url TEXT,
		views INTEGER DEFAULT 0,
		is_active BOOLEAN DEFAULT 1,
		deleted_at DATETIME
	)`).Error)
	assert.NoError(t, db.Exec(`INSERT INTO videos
		(beauty_center_id, title, description, file_path, thumbnail_url, views, deleted_at)
		VALUES
		(1, 'Corte fade', 'Paso a paso', '/videos/stream/a.mp4', '/thumbs/a.jpg', 250, NULL),
		(1, 'Roto sin archivo', '', '', '', 0, NULL)`).Error)

	configs.BackfillLegacyVideos()

	var count int64
	assert.NoError(t, db.Model(&models.FeedPost{}).Count(&count).Error)
	assert.Equal(t, int64(1), count, "invalid row skipped, valid one migrated")

	var post models.FeedPost
	assert.NoError(t, db.First(&post).Error)
	assert.Equal(t, uint(1), post.BeautyCenterID)
	assert.Equal(t, "/videos/stream/a.mp4", post.VideoURL)
	assert.Equal(t, 250, post.ViewsCount)
	assert.True(t, post.IsActive)

	exists := db.Migrator().HasTable("videos")
	assert.False(t, exists, "legacy table dropped after successful backfill")
}
