package service

import (
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
)

// TimeBlockService manages unavailability blocks (breaks, holidays) for a
// professional or the whole centre.
//
// Authorization is by ROLE + ownership:
//   - personnel creates/deletes only blocks on THEMSELVES; the actor id is
//     forced from the JWT, never trusted from the body.
//   - beauty_center creates blocks on any professional of its own centre, or a
//     centre-wide block, and deletes any block of its centre.
type TimeBlockService interface {
	Create(req request.CreateTimeBlockRequest, role string, actorID, centerID uint) (*models.TimeBlock, error)
	// List returns blocks of the acting centre in [from, to]. A staff caller
	// sees only their own + centre-wide blocks.
	List(role string, actorID, centerID uint, from, to string, personnelID *uint) ([]models.TimeBlock, error)
	Delete(id uint, role string, actorID, centerID uint) error
}
