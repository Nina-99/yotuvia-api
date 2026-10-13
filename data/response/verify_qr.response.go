package response

// VerifyQRResponse is the success payload of GET /reservation/verify-qr. It
// carries the reservation DTO plus an already_completed flag so a re-scan of an
// already-completed reservation renders an idempotent state instead of a
// second charge (design D3). Reward is the batched earn outcome of this scan
// (nil when no punch was minted — loyalty disabled, no program, or re-scan).
type VerifyQRResponse struct {
	Reservation      ReservationResponse `json:"reservation"`
	AlreadyCompleted bool                `json:"already_completed"`
	Reward           *RewardLineResponse `json:"reward,omitempty"`
}
