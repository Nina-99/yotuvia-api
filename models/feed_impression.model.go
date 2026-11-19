package models

import "time"

// FeedImpression is one post SERVED to one client at one position.
//
// It exists for a single reason: without knowing what was shown and where, no
// ranking change can ever be evaluated. Every "the feed feels better" argument is
// an opinion until there is a position and an engagement rate behind it.
//
// Written only for the FIRST page (and a bounded number of rows) so the cost is
// one batch insert per feed session, not one row per scroll.
type FeedImpression struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`

	PostID uint `gorm:"not null;index:idx_feed_impression_post_user" json:"post_id"`
	// ClientID is the viewer (polymorphic user id, always role "client" here).
	ClientID uint `gorm:"not null;index:idx_feed_impression_post_user" json:"client_id"`
	// Position is the 1-based slot the viewer saw it in, AFTER the ad merge, so
	// the metric reflects the real screen.
	Position int `gorm:"not null;index" json:"position"`
	// SessionID groups one feed session; it is the ranking seed's other half.
	SessionID string `gorm:"size:64;index" json:"session_id"`
}
