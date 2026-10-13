package service

import (
	"mime/multipart"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
)

type FeedPostService interface {
	CreateFromUpload(actor FeedActor, req request.CreateFeedPostRequest, file *multipart.FileHeader) (response.FeedPostResponse, error)
	// CreateFromExternal links an already-published external video instead of
	// uploading one. Metadata is enriched best-effort via oEmbed; a failed
	// lookup still stores the bare link.
	CreateFromExternal(actor FeedActor, req request.CreateExternalFeedPostRequest) (response.FeedPostResponse, error)
	Update(actor FeedActor, id uint, req request.UpdateFeedPostRequest) (response.FeedPostResponse, error)
	Delete(actor FeedActor, id uint) error
	SetSuspended(centerID, postID uint, suspend bool, note string) error
	IncrementViews(id uint) error
	// IncrementBookTaps records one book-CTA tap; mirrors IncrementViews.
	IncrementBookTaps(id uint) error
	FindAll(page, limit int, clientID *uint) ([]response.FeedPostResponse, int64, error)
	// FindPersonnelFeed serves the public professional-profile feed (active
	// posts only, newest first) with the same body shape and enrichment as
	// FindAll.
	FindPersonnelFeed(personnelID uint, page, limit int, clientID *uint) ([]response.FeedPostResponse, int64, error)
	FindMine(actor FeedActor) ([]response.FeedPostResponse, error)
	FindByBeautyCenterID(centerID uint) ([]response.FeedPostResponse, error)
}
