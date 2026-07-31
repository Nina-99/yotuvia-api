package request

type CreateTopUpRequest struct {
	AmountBs   float64 `json:"amount_bs" validate:"required,min=1"`
	ReceiptURL string  `json:"receipt_url" validate:"required,url"`
}

type ApproveTopUpRequest struct {
	Status string `json:"status" validate:"required,oneof=approved rejected"`
	Notes  string `json:"notes"`
}

type ClaimMissionRequest struct {
	MissionID uint `json:"mission_id" validate:"required"`
}

type CreateReferralRequest struct {
	ReferredCode string `json:"referred_code" validate:"required"`
}

type CreateReferralByCodeRequest struct {
	Code string `json:"code" validate:"required,min=6,max=10"`
}
