package service

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
)

type BeautyCenterServiceImpl struct {
	repo           repository.BeautyCenterRepository
	missionService TokenMissionService
	Validate       *validator.Validate
}

func NewBeautyCenterServiceImpl(repo repository.BeautyCenterRepository, missionService TokenMissionService, validate *validator.Validate) BeautyCenterService {
	return &BeautyCenterServiceImpl{
		repo:           repo,
		missionService: missionService,
		Validate:       validate,
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
func (s *BeautyCenterServiceImpl) Update(id uint, req request.UpdateBeautyCenterRequest) (response.BeautyCenterResponse, error) {
	if err := s.Validate.Struct(req); err != nil {
		return response.BeautyCenterResponse{}, err
	}
	// Find existing
	center, err := s.repo.FindByID(id)
	if err != nil {
		return response.BeautyCenterResponse{}, errors.New("beauty center not found")
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
	if !transitioned || referrerID == nil || s.missionService == nil {
		return nil
	}
	if err := s.missionService.TriggerCheck(*referrerID, "referral"); err != nil {
		utils.Log.Errorf("[MISSION] referral trigger failed for referrer %d: %v", *referrerID, err)
	}
	return nil
}

// Delete implements BeautyCenterService
func (s *BeautyCenterServiceImpl) Delete(id uint) error {
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

	// Get existing reservations for that date (excluding CANCELLED)
	reservations, err := s.repo.FindReservationsForDate(centerID, parsedDate, personnelID)
	if err != nil {
		return response.AvailableSlotsResponse{}, err
	}

	// Helper to check if a time range overlaps with any reservation
	isBooked := func(slotStart, slotEnd int) bool {
		for _, r := range reservations {
			rStart := parseMinutes(r.StartTime)
			rEnd := parseMinutes(r.EndTime)
			if slotStart < rEnd && slotEnd > rStart {
				return true
			}
		}
		return false
	}

	// Get current time to filter past slots on today's date
	now := time.Now()
	nowMinutes := now.Hour()*60 + now.Minute()
	isToday := parsedDate.Year() == now.Year() && parsedDate.YearDay() == now.YearDay()

	stepMinutes := 30
	slotStep := stepMinutes

	// Generate slots: each candidate start time must have `duration` free minutes ahead
	var slots []response.AvailableSlot
	for _, schedule := range schedules {
		startMinutes := parseMinutes(schedule.StartTime)
		endMinutes := parseMinutes(schedule.EndTime)

		for m := startMinutes; m+slotStep <= endMinutes; m += slotStep {
			timeStr := fmt.Sprintf("%02d:%02d", m/60, m%60)

			// Slot is only feasible if the full duration fits within this block
			slotEnd := m + duration
			if slotEnd > endMinutes {
				// Duration doesn't fully fit → mark as unavailable
				slots = append(slots, response.AvailableSlot{
					Time:      timeStr,
					Available: false,
					Reason:    "closing",
				})
				continue
			}

			available := !isBooked(m, slotEnd)
			if isToday && m < nowMinutes {
				available = false
			}
			reason := ""
			if !available && isBooked(m, m+duration) {
				reason = "booked"
			} else if !available && isToday && m < nowMinutes {
				reason = "past"
			}
			slots = append(slots, response.AvailableSlot{
				Time:      timeStr,
				Available: available,
				Reason:    reason,
			})
		}
	}

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
		if parseMinutes(s.StartTime) >= parseMinutes(s.EndTime) {
			return fmt.Errorf("start_time %s must be before end_time %s", s.StartTime, s.EndTime)
		}
		byDay[s.DayOfWeek] = append(byDay[s.DayOfWeek], s)
	}

	// Check for overlapping blocks per day
	for day, blocks := range byDay {
		sort.Slice(blocks, func(i, j int) bool {
			return parseMinutes(blocks[i].StartTime) < parseMinutes(blocks[j].StartTime)
		})
		for i := 1; i < len(blocks); i++ {
			prevEnd := parseMinutes(blocks[i-1].EndTime)
			currStart := parseMinutes(blocks[i].StartTime)
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

// parseMinutes convierte "HH:MM" a minutos desde medianoche.
func parseMinutes(timeStr string) int {
	parts := strings.Split(timeStr, ":")
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	return h*60 + m
}

// Internal mapper
func (s *BeautyCenterServiceImpl) mapToResponse(center models.BeautyCenter) response.BeautyCenterResponse {
	resp := response.BeautyCenterResponse{
		ID:          center.ID,
		Name:        center.Name,
		Address:     center.Address,
		Email:       center.Email,
		Phone:       center.Phone,
		Url:         center.Url,
		Description: center.Description,
		Latitude:    center.Latitude,
		Longitude:   center.Longitude,
		IsActive:    center.IsActive,
		AvgRating:   center.AvgRating,
		ReviewCount: center.ReviewCount,
		Currency:    center.Currency,
		SocialMedia: mapSocialMedia(center.SocialMedia),
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
