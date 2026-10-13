package service

import (
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
)

type BeautyCenterService interface {
	// Update/Delete take the caller's centerID claim and reject a path id that
	// does not match it (apperrors.ErrForbidden), so a center-A admin cannot
	// rewrite or delete center B.
	Update(id, actorCenterID uint, req request.UpdateBeautyCenterRequest) (response.BeautyCenterResponse, error)
	Delete(id, actorCenterID uint) error
	FindByID(id uint) (response.BeautyCenterResponse, error)
	FindAll(page, limit int) ([]response.BeautyCenterResponse, int64, error)
	GetAvailableSlots(centerID uint, date string, duration int, personnelID uint) (response.AvailableSlotsResponse, error)
	Search(query string, page, limit int) ([]response.BeautyCenterResponse, int64, error)
	FindTopRated(limit int) ([]response.BeautyCenterResponse, error)
	FindNearby(latitude, longitude float64, limit int) ([]response.BeautyCenterResponse, error)
	// MarkVerified stamps platform-ops verification (feat-token-missions D5)
	// and fires the referrer's referral mission when a referral links the
	// center. Idempotent no-op for already-verified centers.
	MarkVerified(id uint) error
	// FindRanking returns beauty centers ranked by composite score for the
	// given period ("week", "month", or "all").
	FindRanking(period string, limit int) ([]response.RankingResponse, error)
}
