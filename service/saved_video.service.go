package service

import "github.com/Nina-99/yotuvia-api/data/response"

type SavedVideoService interface {
	/** Toggle save: saves if not saved, removes if already saved. Returns new saved state. */
	Toggle(clientID uint, postID uint) (bool, error)
	/** Check if the client has saved this video. */
	CheckIfSaved(clientID uint, postID uint) (bool, error)
	/** List all saved video post IDs for a client. */
	ListByClient(clientID uint) ([]response.SavedVideoResponse, error)
}
