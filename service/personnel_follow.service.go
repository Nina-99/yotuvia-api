package service

// PersonnelFollowService manages client -> professional follow edges.
// Both Follow and Unfollow are idempotent. The target must be an active
// professional; a missing or inactive professional yields the project's
// canonical ErrNotFound, never a silent success.
type PersonnelFollowService interface {
	Follow(clientID, personnelID uint) error
	Unfollow(clientID, personnelID uint) error
	IsFollowing(clientID, personnelID uint) (bool, error)
}
