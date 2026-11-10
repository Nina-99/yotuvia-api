package service

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
)

type BeautyCenterServiceImpl struct {
	repo           repository.BeautyCenterRepository
	missionService TokenMissionService
	Validate       *validator.Validate
	// timeBlockRepo backs unavailability blocks in GetAvailableSlots. OPTIONAL:
	// nil disables block subtraction.
	timeBlockRepo repository.TimeBlockRepository
	// referralService credits pending referrals when a center is verified.
	// OPTIONAL: nil skips referral crediting (mission trigger still runs).
	referralService ReferralService
}

func NewBeautyCenterServiceImpl(repo repository.BeautyCenterRepository, missionService TokenMissionService, validate *validator.Validate, timeBlockRepo repository.TimeBlockRepository, referralSvc ...ReferralService) BeautyCenterService {
	var referral ReferralService
	if len(referralSvc) > 0 {
		referral = referralSvc[0]
	}
	return &BeautyCenterServiceImpl{
		repo:            repo,
		missionService:  missionService,
		Validate:        validate,
		timeBlockRepo:   timeBlockRepo,
		referralService: referral,
	}
}

// profileComplete evaluates the feat-token-missions D4 checklist: name,
// description, address, phone, at least one schedule and at least one image.
func (s *BeautyCenterServiceImpl) profileComplete(center *models.BeautyCenter) (bool, error) {
	basics := center.Name != "" &&
		center.Description != "" &&
		center.Address != "" &&
		center.Phone != ""
	if !basics {
		return false, nil
	}
	hasSchedules, err := s.repo.HasSchedules(center.ID)
	if err != nil {
		return false, err
	}
	if !hasSchedules {
		return false, nil
	}
	return s.repo.HasImages(center.ID)
}

// Update implements BeautyCenterService
func (s *BeautyCenterServiceImpl) Update(id, actorCenterID uint, req request.UpdateBeautyCenterRequest) (response.BeautyCenterResponse, error) {
	// Ownership first: a centre only ever edits itself. The path id must equal
	// the caller's centerID claim, so a center-A admin cannot rewrite center B
	// (email/password included) and a token without a centerID claim (0) owns
	// nothing. Enforced in the service, like loyalty/promotion/reservation.
	if actorCenterID == 0 || id != actorCenterID {
		return response.BeautyCenterResponse{}, apperrors.ErrForbidden
	}
	if err := s.Validate.Struct(req); err != nil {
		return response.BeautyCenterResponse{}, err
	}
	// Find existing
	center, err := s.repo.FindByID(id)
	if err != nil {
		return response.BeautyCenterResponse{}, apperrors.ErrNotFound
	}

	// feat-token-missions D4: capture completeness BEFORE the update so the
	// profile_completed mission fires only on an incomplete→complete
	// transition, never on repeated updates of an already-complete profile.
	wasComplete, err := s.profileComplete(center)
	if err != nil {
		return response.BeautyCenterResponse{}, err
	}

	// Update basic fields
	if req.Name != "" {
		center.Name = req.Name
	}
	if req.Address != "" {
		center.Address = req.Address
	}
	if req.Phone != "" {
		center.Phone = req.Phone
	}
	if req.Url != "" {
		center.Url = req.Url
	}
	if req.Description != "" {
		center.Description = req.Description
	}
	if req.NameI18n != nil {
		center.NameI18n = utils.NormalizeI18n(req.NameI18n)
	}
	if req.DescriptionI18n != nil {
		center.DescriptionI18n = utils.NormalizeI18n(req.DescriptionI18n)
	}
	if req.Currency != "" {
		center.Currency = req.Currency
	}
	if req.IsActive != nil {
		center.IsActive = *req.IsActive
	}
	if req.Latitude != nil {
		center.Latitude = *req.Latitude
	}
	if req.Longitude != nil {
		center.Longitude = *req.Longitude
	}
	if req.DepositRequired != nil {
		center.DepositRequired = *req.DepositRequired
	}
	if req.DepositAmount != nil {
		center.DepositAmount = *req.DepositAmount
	}
	if req.DepositPercent != nil {
		center.DepositPercent = *req.DepositPercent
	}
	if req.DepositTrustThreshold != nil {
		center.DepositTrustThreshold = *req.DepositTrustThreshold
	}
	if req.DepositHoldMinutes != nil {
		center.DepositHoldMinutes = *req.DepositHoldMinutes
	}
	if req.DepositQRURL != nil {
		center.DepositQRURL = *req.DepositQRURL
	}

	if req.Email != "" && req.Email != center.Email {
		existingEmail, err := s.repo.FindByEmail(req.Email)
		if err != nil {
			utils.Log.Errorf("Update failed: error checking email %s: %v", req.Email, err)
			return response.BeautyCenterResponse{}, err
		}
		if existingEmail != nil {
			utils.Log.Warnf("Update failed: email %s already exists", req.Email)
			return response.BeautyCenterResponse{}, errors.New("email already exists")
		}
		center.Email = req.Email
	}
	if req.Password != "" {
		hashed, err := utils.HashPassword(req.Password)
		if err != nil {
			utils.Log.Errorf("Update failed: failed to hash password: %v", err)
			return response.BeautyCenterResponse{}, err
		}

		center.Password = hashed
	}

	// Update images (Replacement strategy)
	if len(req.Images) > 0 {
		var newImages []models.BeautyCenterImage
		for _, imgReq := range req.Images {
			newImages = append(newImages, models.BeautyCenterImage{
				BeautyCenterID: center.ID,
				ImageUrl:       imgReq.ImageUrl,
				IsCover:        imgReq.IsCover,
				Position:       imgReq.Position,
			})
		}
		if err := s.repo.ReplaceImages(center.ID, newImages); err != nil {
			return response.BeautyCenterResponse{}, err
		}
	}

	// Update schedules (Replacement strategy)
	if len(req.Schedules) > 0 {
		if err := validateSchedules(req.Schedules); err != nil {
			return response.BeautyCenterResponse{}, err
		}

		var newSchedules []models.WorkingHour
		for _, schedReq := range req.Schedules {
			newSchedules = append(newSchedules, models.WorkingHour{
				BeautyCenterID: center.ID,
				DayOfWeek:      schedReq.DayOfWeek,
				StartTime:      schedReq.StartTime,
				EndTime:        schedReq.EndTime,
			})
		}
		if err := s.repo.ReplaceSchedules(center.ID, newSchedules); err != nil {
			return response.BeautyCenterResponse{}, err
		}
	}

	// Save basic fields (without associations)
	center.WorkingHour = nil
	center.BeautyCenterImage = nil
	if err := s.repo.Update(center); err != nil {
		return response.BeautyCenterResponse{}, err
	}

	// feat-token-missions D4/D6: fire profile_completed only on the
	// incomplete→complete transition. Best-effort — never fails the update.
	if !wasComplete {
		if nowComplete, err := s.profileComplete(center); err == nil && nowComplete && s.missionService != nil {
			if err := s.missionService.TriggerCheck(center.ID, "profile_completed"); err != nil {
				utils.Log.Errorf("[MISSION] profile_completed trigger failed for center %d: %v",
					center.ID, err)
			}
		}
	}

	return s.mapToResponse(*center), nil
}

// MarkVerified stamps platform-ops verification (feat-token-missions D5) and,
// when an active referral links the center, fires the referrer's referral
// mission best-effort after the transition commits. Idempotent: already
// verified centers return without side effects.
func (s *BeautyCenterServiceImpl) MarkVerified(id uint) error {
	transitioned, referrerID, err := s.repo.VerifyCenter(id)
	if err != nil {
		return err
	}
	if !transitioned {
		return nil
	}

	// Credit the referral exactly once, atomically, when the referred center is
	// verified. Best-effort: a credit failure logs and leaves it pending for a
	// retry; verification itself already committed.
	if s.referralService != nil {
		if err := s.referralService.ProcessReferralReward(id); err != nil {
			utils.Log.Errorf("[REFERRAL] credit failed for verified center %d: %v", id, err)
		}
	}

	// The referrer's mission fires on the same transition, exactly once.
	if referrerID != nil && s.missionService != nil {
		if err := s.missionService.TriggerCheck(*referrerID, "referral"); err != nil {
			utils.Log.Errorf("[MISSION] referral trigger failed for referrer %d: %v", *referrerID, err)
		}
	}
	return nil
}

// Delete implements BeautyCenterService. Ownership is enforced against the
// caller's centerID claim before the row is touched.
func (s *BeautyCenterServiceImpl) Delete(id, actorCenterID uint) error {
	if actorCenterID == 0 || id != actorCenterID {
		return apperrors.ErrForbidden
	}
	return s.repo.Delete(id)
}

// FindByID implements BeautyCenterService
func (s *BeautyCenterServiceImpl) FindByID(id uint) (response.BeautyCenterResponse, error) {
	center, err := s.repo.FindByID(id)
	if err != nil {
		return response.BeautyCenterResponse{}, err
	}
	return s.mapToResponse(*center), nil
}

// FindAll implements BeautyCenterService
func (s *BeautyCenterServiceImpl) FindAll(page, limit int) ([]response.BeautyCenterResponse, int64, error) {
	centers, total, err := s.repo.FindAll(page, limit)
	if err != nil {
		return nil, total, err
	}

	var resps []response.BeautyCenterResponse
	for _, c := range centers {
		resps = append(resps, s.mapToResponse(c))
	}
	return resps, total, nil
}

// GetAvailableSlots implements BeautyCenterService
func (s *BeautyCenterServiceImpl) GetAvailableSlots(centerID uint, date string, duration int, personnelID uint) (response.AvailableSlotsResponse, error) {
	center, err := s.repo.FindByID(centerID)
	if err != nil {
		return response.AvailableSlotsResponse{}, errors.New("beauty center not found")
	}

	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return response.AvailableSlotsResponse{}, errors.New("invalid date format, use YYYY-MM-DD")
	}
	dayOfWeek := int(parsedDate.Weekday()) // 0=Sun

	// Get working hours for that day (multiple blocks possible)
	schedules, err := s.repo.FindWorkingHours(centerID, dayOfWeek)
	if err != nil {
		return response.AvailableSlotsResponse{}, err
	}
	if len(schedules) == 0 {
		return response.AvailableSlotsResponse{Date: date, Currency: center.Currency, Slots: []response.AvailableSlot{}}, nil
	}

	// Get existing reservations for that date (excluding CANCELLED). When a
	// professional is requested, the busy set is refined to THEIR segments only,
	// so a shared reservation blocks just the professional's own line.
	reservations, err := s.repo.FindReservationsForDate(centerID, parsedDate, personnelID)
	if err != nil {
		return response.AvailableSlotsResponse{}, err
	}
	reservations = PersonnelBusyReservations(reservations, personnelID)

	// Unavailability blocks (breaks/holidays) are folded into the SAME busy set,
	// so a blocked window is never offered. Centre-wide blocks apply to every
	// professional; a professional's own block only to them.
	if s.timeBlockRepo != nil {
		timeBlocks, err := s.timeBlockRepo.ListByCentersRange([]uint{centerID}, parsedDate, parsedDate)
		if err != nil {
			return response.AvailableSlotsResponse{}, err
		}
		reservations = append(reservations, BlockBusyReservations(timeBlocks, personnelID)...)
	}

	// The slot kernel is shared with the cross-center search so the two can
	// never disagree (see service/availability).
	slots := ComputeAvailabilitySlots(parsedDate, duration, schedules, reservations, time.Now())

	return response.AvailableSlotsResponse{Date: date, Currency: center.Currency, Slots: slots}, nil
}

// validateSchedules valida formato HH:MM, StartTime < EndTime, y sin solapamientos por día.
func validateSchedules(schedules []request.WorkingHourRequest) error {
	timeRE := regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
	byDay := make(map[int][]request.WorkingHourRequest)

	for _, s := range schedules {
		if !timeRE.MatchString(s.StartTime) {
			return fmt.Errorf("invalid start_time format %q: expected HH:MM (00:00-23:59)", s.StartTime)
		}
		if !timeRE.MatchString(s.EndTime) {
			return fmt.Errorf("invalid end_time format %q: expected HH:MM (00:00-23:59)", s.EndTime)
		}
		if ParseScheduleMinutes(s.StartTime) >= ParseScheduleMinutes(s.EndTime) {
			return fmt.Errorf("start_time %s must be before end_time %s", s.StartTime, s.EndTime)
		}
		byDay[s.DayOfWeek] = append(byDay[s.DayOfWeek], s)
	}

	// Check for overlapping blocks per day
	for day, blocks := range byDay {
		sort.Slice(blocks, func(i, j int) bool {
			return ParseScheduleMinutes(blocks[i].StartTime) < ParseScheduleMinutes(blocks[j].StartTime)
		})
		for i := 1; i < len(blocks); i++ {
			prevEnd := ParseScheduleMinutes(blocks[i-1].EndTime)
			currStart := ParseScheduleMinutes(blocks[i].StartTime)
			if currStart < prevEnd {
				return fmt.Errorf(
					"overlapping schedules on day %d: %s-%s and %s-%s",
					day,
					blocks[i-1].StartTime, blocks[i-1].EndTime,
					blocks[i].StartTime, blocks[i].EndTime,
				)
			}
		}
	}

	return nil
}

// Internal mapper
func (s *BeautyCenterServiceImpl) mapToResponse(center models.BeautyCenter) response.BeautyCenterResponse {
	resp := response.BeautyCenterResponse{
		ID:                    center.ID,
		Name:                  center.Name,
		Address:               center.Address,
		Email:                 center.Email,
		Phone:                 center.Phone,
		Url:                   center.Url,
		Description:           center.Description,
		NameI18n:              center.NameI18n,
		DescriptionI18n:       center.DescriptionI18n,
		Latitude:              center.Latitude,
		Longitude:             center.Longitude,
		IsActive:              center.IsActive,
		AvgRating:             center.AvgRating,
		ReviewCount:           center.ReviewCount,
		Currency:              center.Currency,
		SocialMedia:           mapSocialMedia(center.SocialMedia),
		DepositRequired:       center.DepositRequired,
		DepositAmount:         center.DepositAmount,
		DepositPercent:        center.DepositPercent,
		DepositTrustThreshold: center.DepositTrustThreshold,
		DepositHoldMinutes:    center.DepositHoldMinutes,
		DepositQRURL:          center.DepositQRURL,
	}

	for _, img := range center.BeautyCenterImage {
		resp.Images = append(resp.Images, response.BeautyCenterImageResp{
			ID:        img.ID,
			ImageUrl:  img.ImageUrl,
			IsCover:   img.IsCover,
			IsProfile: img.IsProfile,
			Position:  img.Position,
		})
	}

	for _, sched := range center.WorkingHour {
		resp.Schedules = append(resp.Schedules, response.WorkingHourResp{
			ID:        sched.ID,
			DayOfWeek: sched.DayOfWeek,
			StartTime: sched.StartTime,
			EndTime:   sched.EndTime,
		})
	}

	return resp
}

func mapSocialMedia(sm []models.SocialMedia) *response.SocialMediaResp {
	if len(sm) == 0 {
		return nil
	}
	return &response.SocialMediaResp{
		Google:    sm[0].Google,
		Facebook:  sm[0].Facebook,
		Instagram: sm[0].Instagram,
		Twitter:   sm[0].Twitter,
		TikTok:    sm[0].TikTok,
	}
}

// Search implements BeautyCenterService
func (s *BeautyCenterServiceImpl) Search(query string, page, limit int) ([]response.BeautyCenterResponse, int64, error) {
	centers, total, err := s.repo.Search(query, page, limit)
	if err != nil {
		return nil, 0, err
	}

	var resps []response.BeautyCenterResponse
	for _, c := range centers {
		resps = append(resps, s.mapToResponse(c))
	}
	return resps, total, nil
}

// FindTopRated implements BeautyCenterService
func (s *BeautyCenterServiceImpl) FindTopRated(limit int) ([]response.BeautyCenterResponse, error) {
	centers, err := s.repo.FindTopRated(limit)
	if err != nil {
		return nil, err
	}

	var resps []response.BeautyCenterResponse
	for _, c := range centers {
		resps = append(resps, s.mapToResponse(c))
	}
	return resps, nil
}

// FindNearby implements BeautyCenterService
func (s *BeautyCenterServiceImpl) FindNearby(latitude, longitude float64, limit int) ([]response.BeautyCenterResponse, error) {
	centers, err := s.repo.FindNearby(latitude, longitude, limit)
	if err != nil {
		return nil, err
	}

	var resps []response.BeautyCenterResponse
	for _, c := range centers {
		resps = append(resps, s.mapToResponse(c))
	}
	return resps, nil
}

// FindRanking implements BeautyCenterService
func (s *BeautyCenterServiceImpl) FindRanking(period string, limit int) ([]response.RankingResponse, error) {
	var from *time.Time

	switch period {
	case "week":
		t := time.Now().UTC().AddDate(0, 0, -7)
		from = &t
	case "month":
		t := time.Now().UTC().AddDate(0, -1, 0)
		from = &t
	case "all":
		from = nil
	default:
		from = nil
	}

	rows, err := s.repo.FindRanking(from, limit)
	if err != nil {
		return nil, err
	}

	var results []response.RankingResponse
	for i, row := range rows {
		// Fetch images for each center.
		var images []models.BeautyCenterImage
		_ = s.repo.(*repository.BeautyCenterRepositoryImpl)
		// Use a simple query to get images.
		var imgModels []models.BeautyCenterImage
		if dbErr := s.repo.(*repository.BeautyCenterRepositoryImpl).DB().
			Where("beauty_center_id = ?", row.CenterID).
			Order("position ASC").
			Find(&imgModels).Error; dbErr == nil {
			images = imgModels
		}

		var imgResps []response.BeautyCenterImageResp
		for _, img := range images {
			imgResps = append(imgResps, response.BeautyCenterImageResp{
				ID:        img.ID,
				ImageUrl:  img.ImageUrl,
				IsCover:   img.IsCover,
				IsProfile: img.IsProfile,
				Position:  img.Position,
			})
		}

		results = append(results, response.RankingResponse{
			Position:       i + 1,
			CenterID:       row.CenterID,
			CenterName:     row.CenterName,
			Address:        row.Address,
			AvgRating:      row.AvgRating,
			ReviewCount:    row.ReviewCount,
			ReservationCnt: row.ReservationCnt,
			CompletionRate: row.CompletionRate,
			VideoViews:     row.VideoViews,
			VideoLikes:     row.VideoLikes,
			FavoritesCount: row.FavoritesCount,
			Score:          row.Score,
			Images:         imgResps,
		})
	}

	return results, nil
}
