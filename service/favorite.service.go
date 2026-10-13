package service

import (
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
)

type FavoriteService interface {
	Create(req request.CreateFavoriteRequest) error
	// Delete removes a favorite only when it belongs to actorClientID (the
	// caller's token claim); a foreign row is ErrForbidden, a missing row is
	// ErrNotFound.
	Delete(id uint, actorClientID uint) error
	// FindByClientID returns a client's favorites only when clientID matches
	// actorClientID (the caller's token claim); a mismatch is ErrForbidden.
	FindByClientID(clientID uint, actorClientID uint) ([]response.FavoriteResponse, error)
	IsFavorite(clientID uint, beautyCenterID uint) (bool, error)
}
