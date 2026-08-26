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
	Validate               *validator.Validate
}

func NewFeedPostServiceImpl(
	feedPostRepo repository.FeedPostRepository,
	beautyCenterRepo repository.BeautyCenterRepository,
	personnelRepo repository.PersonnelRepository,
	serviceRepo repository.ServiceRepository,
	validate *validator.Validate,
) FeedPostService {
	return &FeedPostServiceImpl{
		FeedPostRepository:     feedPostRepo,
		BeautyCenterRepository: beautyCenterRepo,
		PersonnelRepository:    personnelRepo,
		ServiceRepository:      serviceRepo,
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

	centerID, personnelID, err := s.resolveAuthorship(actor, req.PersonnelID)
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
		VideoURL:       "/videos/stream/" + fileName,
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
	if req.IsPromo {
		post.IsPromo = true
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
	if post.PersonnelID == 0 {
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
func (s *FeedPostServiceImpl) FindAll(page, limit int) ([]response.FeedPostResponse, int64, error) {
	posts, total, err := s.FeedPostRepository.FindAllActive(page, limit)
	if err != nil {
		return nil, total, err
	}
	res := make([]response.FeedPostResponse, 0, len(posts))
	for _, post := range posts {
		res = append(res, s.mapToResponse(post))
	}
	return res, total, nil
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
		return post.PersonnelID != 0 && post.PersonnelID == actor.UserID
	}
	return actor.CenterID != 0 && post.BeautyCenterID == actor.CenterID && post.PersonnelID == 0
}
