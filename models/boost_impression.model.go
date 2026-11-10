package models

import "time"

// BoostImpression is the billing dedupe row for one qualified view of a
// promoted post. The triple (campaign, viewer, nonce) is unique: a replayed or
// retried report can never bill the same impression twice. The row is written in
// the same transaction as the token debit and the campaign spend.
type BoostImpression struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	CampaignID uint   `gorm:"not null;index;uniqueIndex:idx_boost_impression_dedupe" json:"campaign_id"`
	ViewerID   uint   `gorm:"not null;index;uniqueIndex:idx_boost_impression_dedupe" json:"viewer_id"`
	Nonce      string `gorm:"size:64;not null;uniqueIndex:idx_boost_impression_dedupe" json:"nonce"`

	// PriceCPMMilli is the signed auction price this impression was billed at.
	PriceCPMMilli int64 `gorm:"type:bigint;not null" json:"price_cpm_milli"`
}
