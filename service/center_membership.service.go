package service

import "github.com/Nina-99/yottu-api/data/request"

type CenterMembershipService interface {
	BuyMembership(centerID uint, req request.BuyMembershipRequest) error
	ReviewMembership(subscriptionID uint, status string) error
	HasActiveAccess(centerID uint) bool
}
