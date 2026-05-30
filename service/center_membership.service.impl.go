package service

import (
	"errors"
	"time"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/go-playground/validator/v10"
)

type CenterMembershipServiceImpl struct {
	membershipRepo       repository.MembershipRepository
	centerMembershipRepo repository.CenterMembershipRepository
	Validate             *validator.Validate
}

func NewCenterMembershipServiceImpl(
	membershipRepo repository.MembershipRepository,
	centerMembershipRepo repository.CenterMembershipRepository,
	validate *validator.Validate,
) CenterMembershipService {
	return &CenterMembershipServiceImpl{
		membershipRepo:       membershipRepo,
		centerMembershipRepo: centerMembershipRepo,
		Validate:             validate,
	}
}

func (s *CenterMembershipServiceImpl) BuyMembership(centerID uint, req request.BuyMembershipRequest) error {
	if err := s.Validate.Struct(req); err != nil {
		return err
	}

	membership, err := s.membershipRepo.FindByID(req.MembershipID)
	if err != nil {
		return errors.New("membership not found")
	}

	now := time.Now()
	// We add up the duration of the days
	endDate := now.AddDate(0, 0, membership.DurationDays)
	// We give 24hs of grace to approve
	provisionalEnd := now.Add(24 * time.Hour)

	newSub := &models.CenterMembership{
		BeautyCenterID:     centerID,
		MembershipID:       req.MembershipID,
		Status:             "provisional",
		ReceiptURL:         req.ReceiptURL,
		StartDate:          now,
		EndDate:            endDate,
		ProvisionalEndDate: provisionalEnd,
	}

	return s.centerMembershipRepo.Save(newSub)
}

func (s *CenterMembershipServiceImpl) ReviewMembership(subscriptionID uint, status string) error {
	sub, err := s.centerMembershipRepo.FindByID(subscriptionID)
	if err != nil {
		return err
	}

	if sub.Status != "provisional" {
		return errors.New("this subscription has already been reviewed")
	}

	return s.centerMembershipRepo.UpdateStatus(subscriptionID, status)
}

func (s *CenterMembershipServiceImpl) HasActiveAccess(centerID uint) bool {
	sub, err := s.centerMembershipRepo.GetLatestByCenterID(centerID)
	if err != nil || sub == nil {
		return false
	}

	now := time.Now()

	if sub.Status == "active" && now.Before(sub.EndDate) {
		return true
	}

	if sub.Status == "provisional" && now.Before(sub.ProvisionalEndDate) {
		return true
	}

	return false
}
