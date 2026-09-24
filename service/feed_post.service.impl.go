package service

import (
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"time"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
)

// Actor identifies WHO is performing a feed write. Identity fields come
// exclusively from JWT claims (feat-video-feed-unification D2) — the client
// never declares authorship.
type FeedActor struct {
	Role     string // "personnel" | "beauty_center"
	UserID   uint   // personnel id when Role==personnel
	CenterID uint   // beauty center id when Role==beauty_center
}

func (a FeedActor) asStaff() bool { return a.Role == "personnel" }

type FeedPostServiceImpl struct {
	FeedPostRepository     repository.FeedPostRepository
	BeautyCenterRepository repository.BeautyCenterRepository
	PersonnelRepository    repository.PersonnelRepository
	ServiceRepository      repository.ServiceRepository
	PostLikeRepository     repository.PostLikeRepository
	SavedVideoRepository   repository.SavedVideoRepository
	CommentRepository      repository.VideoCommentRepository
	Validate               *validator.Validate
}

// NewFeedPostServiceImpl builds the feed service. Every dependency below is
// REQUIRED: FindAll dereferences FeedPost/PostLike/SavedVideo/Comment, the
// write paths dereference Personnel/Service, and the feed is public, so there
// is no reachable configuration in which any of them may be nil. requireDeps
// fails bootstrap instead of letting a nil surface as a request-time 500.
func NewFeedPostServiceImpl(
	feedPostRepo repository.FeedPostRepository,
	beautyCenterRepo repository.BeautyCenterRepository,
	personnelRepo repository.PersonnelRepository,
	serviceRepo repository.ServiceRepository,
	postLikeRepo repository.PostLikeRepository,
	savedVideoRepo repository.SavedVideoRepository,
	commentRepo repository.VideoCommentRepository,
	validate *validator.Validate,
) FeedPostService {
	requireDeps("NewFeedPostServiceImpl",
		dep("FeedPostRepository", feedPostRepo),
		dep("BeautyCenterRepository", beautyCenterRepo),
		dep("PersonnelRepository", personnelRepo),
		dep("ServiceRepository", serviceRepo),
		dep("PostLikeRepository", postLikeRepo),
		dep("SavedVideoRepository", savedVideoRepo),
		dep("CommentRepository", commentRepo),
		dep("Validate", validate),
	)
	return &FeedPostServiceImpl{
		FeedPostRepository:     feedPostRepo,
		BeautyCenterRepository: beautyCenterRepo,
		PersonnelRepository:    personnelRepo,
		ServiceRepository:      serviceRepo,
		PostLikeRepository:     postLikeRepo,
		SavedVideoRepository:   savedVideoRepo,
		CommentRepository:      commentRepo,
		Validate:               validate,
	}
}

// validateMedia mirrors the legacy upload rules: mp4/mov/webm up to 100MB.
func validateMedia(file *multipart.FileHeader) error {
	const maxBytes = 100 * 1024 * 1024
	if file.Size > maxBytes {
		return errors.New("file too large: max 100MB")
	}
	switch fileExtOf(file.Filename) {
	case ".mp4", ".mov", ".webm":
		return nil
	default:
		return errors.New("invalid file extension: only mp4, mov or webm")
	}
}

func fileExtOf(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[i:]
		}
	}
	return ""
}

// resolveAuthorship derives center/author ids from the actor per D2.
func (s *FeedPostServiceImpl) resolveAuthorship(actor FeedActor, creditedPersonnelID uint) (uint, uint, error) {
	if actor.asStaff() {
		staff, findErr := s.PersonnelRepository.FindByID(actor.UserID)
		if findErr != nil || staff == nil {
			return 0, 0, errors.New("staff profile not found")
		}
		if staff.BeautyCenterID == 0 {
			return 0, 0, errors.New("staff has no center")
		}
		return staff.BeautyCenterID, actor.UserID, nil
	}

	if actor.CenterID == 0 {
		return 0, 0, errors.New("missing center claim")
	}
	centerID := actor.CenterID
	if creditedPersonnelID != 0 {
		staff, findErr := s.PersonnelRepository.FindByID(creditedPersonnelID)
		if findErr != nil || staff == nil || staff.BeautyCenterID != centerID {
			return 0, 0, errors.New("credited staff does not belong to your center")
		}
	}
	return centerID, creditedPersonnelID, nil
}

// CreateFromUpload persists a video post from a multipart upload.
func (s *FeedPostServiceImpl) CreateFromUpload(actor FeedActor, req request.CreateFeedPostRequest, file *multipart.FileHeader) (response.FeedPostResponse, error) {
	if req.Caption == "" && req.ServiceID == 0 && !req.IsPromo {
		return response.FeedPostResponse{}, errors.New("caption, service or promo flag required")
	}
	if err := validateMedia(file); err != nil {
		return response.FeedPostResponse{}, err
	}

	centerID, rawPersonnelID, err := s.resolveAuthorship(actor, req.PersonnelID)
	if err != nil {
		return response.FeedPostResponse{}, err
	}
	var serviceID *uint
	if req.ServiceID != 0 {
		svcRow, svcErr := s.ServiceRepository.FindById(req.ServiceID)
		if svcErr != nil || svcRow == nil || svcRow.BeautyCenterID != centerID {
			return response.FeedPostResponse{}, errors.New("service does not belong to your center")
		}
		serviceID = &req.ServiceID
	}
	var personnelID *uint
	if rawPersonnelID != 0 {
		personnelID = &rawPersonnelID
	}

	timestamp := time.Now().UnixNano()
	fileName := fmt.Sprintf("%d_%d%s", centerID, timestamp, fileExtOf(file.Filename))
	filePath, err := saveFeedVideo(file, fileName)
	if err != nil {
		return response.FeedPostResponse{}, err
	}

	post := models.FeedPost{
		BeautyCenterID: centerID,
		PersonnelID:    personnelID,
		ServiceID:      serviceID,
		VideoURL:       "/uploads/videos/" + fileName,
		Caption:        req.Caption,
		IsPromo:        req.IsPromo,
		IsActive:       true,
	}
	if err := s.FeedPostRepository.Create(&post); err != nil {
		utils.Log.Errorf("[FEED] removing orphaned file %s after create failure: %v", filePath, err)
		_ = utils.RemoveFile(filePath)
		return response.FeedPostResponse{}, err
	}

	utils.Log.Infof("[FEED] post created by %s %d (center %d): %s", actor.Role, actor.UserID, centerID, post.Caption)
	return s.mapToResponse(post), nil
}

// Update allows ONLY the author to edit caption/promo flags of their own post.
func (s *FeedPostServiceImpl) Update(actor FeedActor, id uint, req request.UpdateFeedPostRequest) (response.FeedPostResponse, error) {
	post, err := s.FeedPostRepository.FindByID(id)
	if err != nil {
		return response.FeedPostResponse{}, err
	}
	if !ownsPost(actor, post) {
		return response.FeedPostResponse{}, errors.New("only the author can update this post")
	}
	if req.Caption != "" {
		post.Caption = req.Caption
	}
	if req.IsPromo != nil {
		post.IsPromo = *req.IsPromo
	}
	if err := s.FeedPostRepository.Update(post); err != nil {
		return response.FeedPostResponse{}, err
	}
	return s.mapToResponse(*post), nil
}

// Delete allows ONLY the author to remove their own post (D3).
func (s *FeedPostServiceImpl) Delete(actor FeedActor, id uint) error {
	post, err := s.FeedPostRepository.FindByID(id)
	if err != nil {
		return err
	}
	if !ownsPost(actor, post) {
		return errors.New("only the author can delete this post")
	}
	return s.FeedPostRepository.Delete(id)
}

// SetSuspended toggles moderation state over a STAFF-authored post of the
// calling center (D3). Empty note re-enables. Center-authored posts cannot be
// suspended through this action.
func (s *FeedPostServiceImpl) SetSuspended(centerID, postID uint, suspend bool, note string) error {
	post, err := s.FeedPostRepository.FindByID(postID)
	if err != nil {
		return err
	}
	if post.BeautyCenterID != centerID {
		return errors.New("post does not belong to your center")
	}
	if post.PersonnelID == nil {
		return errors.New("center-authored posts cannot be suspended")
	}
	post.IsActive = !suspend
	if suspend {
		post.ModerationNote = note
	} else {
		post.ModerationNote = ""
	}
	return s.FeedPostRepository.Update(post)
}

// IncrementViews bumps the view counter; failures are swallowed upstream.
func (s *FeedPostServiceImpl) IncrementViews(id uint) error {
	return s.FeedPostRepository.IncrementViews(id)
}

// FindAll serves the PUBLIC feed: active posts only, newest first.
func (s *FeedPostServiceImpl) FindAll(page, limit int, clientID *uint) ([]response.FeedPostResponse, int64, error) {
	posts, total, err := s.FeedPostRepository.FindAllActive(page, limit)
	if err != nil {
		return nil, total, err
	}

	// Pre-fetch user's likes and saves for batch (avoid N+1)
	likedPostIDs := make(map[uint]bool)
	savedPostIDs := make(map[uint]bool)
	if clientID != nil {
		if likes, err := s.PostLikeRepository.FindByClientID(*clientID); err == nil {
			for _, l := range likes {
				likedPostIDs[l.FeedPostID] = true
			}
		}
		if saves, err := s.SavedVideoRepository.FindByClientID(*clientID); err == nil {
			for _, sv := range saves {
				savedPostIDs[sv.FeedPostID] = true
			}
		}
	}

	res := make([]response.FeedPostResponse, 0, len(posts))
	for _, post := range posts {
		resp := s.mapToResponse(post)
		resp.IsLiked = likedPostIDs[post.ID]
		resp.IsSaved = savedPostIDs[post.ID]
		res = append(res, resp)
	}

	// One grouped COUNT per page instead of one COUNT per post (N+1).
	s.attachCommentCounts(res)
	s.attachRealLikeCounts(res)
	return res, total, nil
}

// attachCommentCounts fills CommentsCount with one grouped query for the whole
// page. Missing ids (no comments) stay at zero; a query error leaves the mapped
// values untouched rather than blanking the whole page.
func (s *FeedPostServiceImpl) attachCommentCounts(res []response.FeedPostResponse) {
	if len(res) == 0 {
		return
	}
	ids := make([]uint, 0, len(res))
	for i := range res {
		ids = append(ids, res[i].ID)
	}
	counts, err := s.CommentRepository.CountByFeedPostIDs(ids)
	if err != nil {
		return
	}
	for i := range res {
		res[i].CommentsCount = int(counts[res[i].ID])
	}
}

// attachRealLikeCounts overwrites the stored FeedPost.LikesCount (vestigial —
// only the seeder ever wrote it) with the real number of live post_likes rows
// for the page. One grouped query keeps the feed endpoints correct without an
// N+1, and the value can never drift from post_likes.
func (s *FeedPostServiceImpl) attachRealLikeCounts(res []response.FeedPostResponse) {
	if len(res) == 0 {
		return
	}
	ids := make([]uint, 0, len(res))
	for i := range res {
		ids = append(ids, res[i].ID)
	}
	counts, err := s.PostLikeRepository.CountByFeedPostIDs(ids)
	if err != nil {
		return
	}
	for i := range res {
		res[i].LikesCount = int(counts[res[i].ID])
	}
}

// FindMine returns every post authored by the actor (staff id or center),
// including suspended ones, for management surfaces.
func (s *FeedPostServiceImpl) FindMine(actor FeedActor) ([]response.FeedPostResponse, error) {
	var (
		posts []models.FeedPost
		err   error
	)
	if actor.asStaff() {
		posts, err = s.FeedPostRepository.FindByPersonnelID(actor.UserID)
	} else {
		posts, err = s.FeedPostRepository.FindAllByCenter(actor.CenterID)
	}
	if err != nil {
		return nil, err
	}
	res := make([]response.FeedPostResponse, 0, len(posts))
	for _, post := range posts {
		res = append(res, s.mapToResponse(post))
	}
	s.attachRealLikeCounts(res)
	return res, nil
}

func (s *FeedPostServiceImpl) FindByBeautyCenterID(centerID uint) ([]response.FeedPostResponse, error) {
	posts, err := s.FeedPostRepository.FindActiveByCenter(centerID)
	if err != nil {
		return nil, err
	}
	res := make([]response.FeedPostResponse, 0, len(posts))
	for _, post := range posts {
		res = append(res, s.mapToResponse(post))
	}
	s.attachRealLikeCounts(res)
	return res, nil
}

func (s *FeedPostServiceImpl) mapToResponse(post models.FeedPost) response.FeedPostResponse {
	resp := response.FeedPostResponse{
		ID:             post.ID,
		BeautyCenterID: post.BeautyCenterID,
		BeautyCenter: response.BeautyCenterResponse{
			ID:      post.BeautyCenter.ID,
			Name:    post.BeautyCenter.Name,
			Address: post.BeautyCenter.Address,
		},
		PersonnelID:    post.PersonnelID,
		ServiceID:      post.ServiceID,
		IsPromo:        post.IsPromo,
		ModerationNote: post.ModerationNote,
		IsActive:       post.IsActive,
		VideoURL:       post.VideoURL,
		ThumbnailURL:   post.ThumbnailURL,
		Caption:        post.Caption,
		Duration:       post.Duration,
		ViewsCount:     post.ViewsCount,
		LikesCount:     post.LikesCount,
		CreatedAt:      post.CreatedAt,
	}
	if post.Service.ID != 0 {
		resp.ServiceName = post.Service.Name
		resp.CategoryID = post.Service.CategoryID
		if post.Service.Category.ID != 0 {
			resp.CategoryName = post.Service.Category.Name
		}
	}
	if post.Personnel.ID != 0 {
		resp.PersonnelName = fmt.Sprintf("%s %s",
			post.Personnel.FirstName, post.Personnel.LastName)
	}
	return resp
}

// saveFeedVideo stores the uploaded file under the shared streaming path
// (uploads/videos) using the same conventions as the legacy pipeline.
func saveFeedVideo(file *multipart.FileHeader, fileName string) (string, error) {
	const uploadDir = "uploads/videos"
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create upload dir: %w", err)
	}
	filePath := uploadDir + "/" + fileName
	if err := utils.SaveUploadedFile(file, filePath); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}
	return filePath, nil
}

// ownsPost resolves authorship for update/delete: staff own posts credited to
// them; centers own the posts they published without staff credit.
func ownsPost(actor FeedActor, post *models.FeedPost) bool {
	if actor.asStaff() {
		return post.PersonnelID != nil && *post.PersonnelID == actor.UserID
	}
	return actor.CenterID != 0 && post.BeautyCenterID == actor.CenterID && post.PersonnelID == nil
}
