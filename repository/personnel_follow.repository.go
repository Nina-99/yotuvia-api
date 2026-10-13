package repository

import "github.com/Nina-99/yotuvia-api/models"

// PersonnelFollowRepository persists client -> professional follow edges.
// Every mutator is idempotent: Follow never duplicates a row (DB unique index
// + OnConflict DoNothing) and Unfollow on a missing edge is a no-op.
type PersonnelFollowRepository interface {
	// Follow inserts the edge, silently ignoring an existing one.
	Follow(follow *models.PersonnelFollow) error
	// Unfollow removes the edge if present; deleting zero rows is not an error.
	Unfollow(clientID, personnelID uint) error
	// IsFollowing reports whether the edge exists.
	IsFollowing(clientID, personnelID uint) (bool, error)
	// CountByPersonnelID returns how many clients follow the professional.
	CountByPersonnelID(personnelID uint) (int64, error)
	// FindFollowerIDsByPersonnelID loads the follower client ids in ONE query,
	// capped at limit (callers pass a fan-out ceiling). Ordered by edge id so
	// the cap is deterministic.
	FindFollowerIDsByPersonnelID(personnelID uint, limit int) ([]uint, error)
}
