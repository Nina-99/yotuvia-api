package service

import (
	"errors"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
)

var (
	ErrFeedPostNotFound    = errors.New("feed post not found")
	ErrCommentNotFound     = errors.New("comment not found")
	ErrMaxDepthExceeded    = errors.New("replies limited to one level deep")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
)

// asClient reports whether the actor is an end client. Clients authenticate
// with role "client" (see middleware/authorize.middleware.go); staff are
// "personnel" and centers are "beauty_center".
func (a FeedActor) asClient() bool { return a.Role == "client" }

type VideoCommentService interface {
	CreateComment(actor FeedActor, feedPostID uint, req request.CreateCommentRequest) (response.CommentResponse, error)
	ReplyToComment(actor FeedActor, commentID uint, req request.ReplyRequest) (response.CommentResponse, error)
	UpdateComment(actor FeedActor, commentID uint, req request.UpdateCommentRequest) (response.CommentResponse, error)
	DeleteComment(actor FeedActor, commentID uint) error
	HideComment(actor FeedActor, commentID uint) error
	ListComments(feedPostID uint, cursor *uint, limit int) ([]response.CommentResponse, *uint, error)
	GetCommentReplies(commentID uint) ([]response.CommentResponse, error)
}

type VideoCommentServiceImpl struct {
	commentRepo  repository.VideoCommentRepository
	feedPostRepo repository.FeedPostRepository
	clientRepo   repository.ClientRepository
}

func NewVideoCommentServiceImpl(
	commentRepo repository.VideoCommentRepository,
	feedPostRepo repository.FeedPostRepository,
	clientRepo repository.ClientRepository,
) VideoCommentService {
	return &VideoCommentServiceImpl{
		commentRepo:  commentRepo,
		feedPostRepo: feedPostRepo,
		clientRepo:   clientRepo,
	}
}

func (s *VideoCommentServiceImpl) CreateComment(actor FeedActor, feedPostID uint, req request.CreateCommentRequest) (response.CommentResponse, error) {
	// Verify feed post exists.
	fp, err := s.feedPostRepo.FindByID(feedPostID)
	if err != nil || fp == nil {
		return response.CommentResponse{}, ErrFeedPostNotFound
	}

	// If replying to a parent, validate depth.
	if req.ParentID != nil {
		parent, err := s.commentRepo.FindByID(*req.ParentID)
		if err != nil || parent == nil {
			return response.CommentResponse{}, ErrCommentNotFound
		}
		if parent.IsHidden {
			return response.CommentResponse{}, ErrCommentNotFound
		}
		if parent.ParentID != nil {
			return response.CommentResponse{}, ErrMaxDepthExceeded
		}
	}

	comment := &models.VideoComment{
		FeedPostID: feedPostID,
		ParentID:   req.ParentID,
		Body:       req.Body,
		IsOfficial: false,
	}
	// Attribute the comment to the authoring client. Clients authenticate with
	// role "client" (never ""), so the old `actor.Role == ""` check never fired
	// and every client comment was persisted with client_id NULL. Staff/center
	// comments carry no client_id by design.
	if actor.asClient() {
		cid := actor.UserID
		comment.ClientID = &cid
	}

	if err := s.commentRepo.Create(comment); err != nil {
		return response.CommentResponse{}, err
	}

	m := s.newCommentMapper([]models.VideoComment{*comment})
	return m.mapComment(comment), nil
}

func (s *VideoCommentServiceImpl) ReplyToComment(actor FeedActor, commentID uint, req request.ReplyRequest) (response.CommentResponse, error) {
	parent, err := s.commentRepo.FindByID(commentID)
	if err != nil || parent == nil {
		return response.CommentResponse{}, ErrCommentNotFound
	}
	if parent.IsHidden {
		return response.CommentResponse{}, ErrCommentNotFound
	}

	// Only center owners can reply officially.
	if actor.CenterID == 0 {
		return response.CommentResponse{}, ErrForbidden
	}

	// The official reply must come from the center that OWNS the feed post.
	// Previously any authenticated center could post an official reply on any
	// other center's video.
	fp, err := s.feedPostRepo.FindByID(parent.FeedPostID)
	if err != nil || fp == nil {
		return response.CommentResponse{}, ErrFeedPostNotFound
	}
	if fp.BeautyCenterID != actor.CenterID {
		return response.CommentResponse{}, ErrUnauthorized
	}

	reply := &models.VideoComment{
		FeedPostID: parent.FeedPostID,
		ParentID:   &commentID,
		Body:       req.Body,
		IsOfficial: true,
	}

	if err := s.commentRepo.Create(reply); err != nil {
		return response.CommentResponse{}, err
	}

	m := s.newCommentMapper([]models.VideoComment{*reply})
	return m.mapComment(reply), nil
}

func (s *VideoCommentServiceImpl) UpdateComment(actor FeedActor, commentID uint, req request.UpdateCommentRequest) (response.CommentResponse, error) {
	comment, err := s.commentRepo.FindByID(commentID)
	if err != nil || comment == nil {
		return response.CommentResponse{}, ErrCommentNotFound
	}

	// Only the original author can edit.
	if comment.ClientID == nil || *comment.ClientID != actor.UserID {
		return response.CommentResponse{}, ErrUnauthorized
	}

	if req.Body != "" {
		comment.Body = req.Body
	}

	if err := s.commentRepo.Update(comment); err != nil {
		return response.CommentResponse{}, err
	}

	m := s.newCommentMapper([]models.VideoComment{*comment})
	return m.mapComment(comment), nil
}

func (s *VideoCommentServiceImpl) DeleteComment(actor FeedActor, commentID uint) error {
	comment, err := s.commentRepo.FindByID(commentID)
	if err != nil || comment == nil {
		return ErrCommentNotFound
	}

	// Owner can always delete their own comment.
	if comment.ClientID != nil && *comment.ClientID == actor.UserID {
		return s.commentRepo.Delete(comment.ID)
	}

	// Center can delete any comment on their own feed posts.
	if actor.CenterID != 0 {
		fp, err := s.feedPostRepo.FindByID(comment.FeedPostID)
		if err == nil && fp != nil && fp.BeautyCenterID == actor.CenterID {
			return s.commentRepo.Delete(comment.ID)
		}
	}

	return ErrUnauthorized
}

func (s *VideoCommentServiceImpl) HideComment(actor FeedActor, commentID uint) error {
	comment, err := s.commentRepo.FindByID(commentID)
	if err != nil || comment == nil {
		return ErrCommentNotFound
	}

	// Only the owning center can hide comments on their feed posts.
	if actor.CenterID == 0 {
		return ErrForbidden
	}

	// Verify the feed post belongs to the actor's center. Previously this only
	// checked that SOME center was acting, so any center could hide any
	// comment anywhere.
	fp, err := s.feedPostRepo.FindByID(comment.FeedPostID)
	if err != nil || fp == nil {
		return ErrFeedPostNotFound
	}
	if fp.BeautyCenterID != actor.CenterID {
		return ErrUnauthorized
	}

	comment.IsHidden = true
	return s.commentRepo.Update(comment)
}

func (s *VideoCommentServiceImpl) ListComments(feedPostID uint, cursor *uint, limit int) ([]response.CommentResponse, *uint, error) {
	if limit <= 0 {
		limit = 20
	}

	comments, err := s.commentRepo.FindByFeedPost(feedPostID, cursor, limit+1)
	if err != nil {
		return nil, nil, err
	}

	var nextCursor *uint
	if len(comments) > limit {
		comments = comments[:limit]
		last := comments[len(comments)-1].ID
		nextCursor = &last
	}

	result := make([]response.CommentResponse, 0, len(comments))
	m := s.newCommentMapper(comments)
	for i := range comments {
		result = append(result, m.mapComment(&comments[i]))
	}
	return result, nextCursor, nil
}

func (s *VideoCommentServiceImpl) GetCommentReplies(commentID uint) ([]response.CommentResponse, error) {
	replies, err := s.commentRepo.FindReplies(commentID)
	if err != nil {
		return nil, err
	}

	result := make([]response.CommentResponse, 0, len(replies))
	m := s.newCommentMapper(replies)
	for i := range replies {
		result = append(result, m.mapComment(&replies[i]))
	}
	return result, nil
}

// commentResponseMapper holds the author/feed-post lookups shared by a batch of
// comments, resolved in one query each. It replaces the old per-comment
// FindById/FindByID N+1 in mapToResponse.
type commentResponseMapper struct {
	clients map[uint]*models.Client
	posts   map[uint]*models.FeedPost
}

// newCommentMapper inspects a page (top-level comments and their direct
// replies), collects the distinct client and feed-post ids, and resolves each
// set with a single FindByIDs. Lookups stay best-effort, matching the old
// per-row behaviour: on error the entry is simply absent and the author renders
// without the unresolved fields.
func (s *VideoCommentServiceImpl) newCommentMapper(comments []models.VideoComment) *commentResponseMapper {
	m := &commentResponseMapper{
		clients: make(map[uint]*models.Client),
		posts:   make(map[uint]*models.FeedPost),
	}

	clientIDs := make(map[uint]struct{})
	postIDs := make(map[uint]struct{})

	var collect func(cs []models.VideoComment)
	collect = func(cs []models.VideoComment) {
		for i := range cs {
			c := &cs[i]
			if c.ClientID != nil {
				clientIDs[*c.ClientID] = struct{}{}
			}
			if c.IsOfficial {
				postIDs[c.FeedPostID] = struct{}{}
			}
			if len(c.Replies) > 0 {
				collect(c.Replies)
			}
		}
	}
	collect(comments)

	if len(clientIDs) > 0 {
		ids := make([]uint, 0, len(clientIDs))
		for id := range clientIDs {
			ids = append(ids, id)
		}
		if clients, err := s.clientRepo.FindByIDs(ids); err == nil {
			for i := range clients {
				m.clients[clients[i].ID] = &clients[i]
			}
		}
	}

	if len(postIDs) > 0 {
		ids := make([]uint, 0, len(postIDs))
		for id := range postIDs {
			ids = append(ids, id)
		}
		if posts, err := s.feedPostRepo.FindByIDs(ids); err == nil {
			for i := range posts {
				m.posts[posts[i].ID] = &posts[i]
			}
		}
	}

	return m
}

// mapComment transforms a domain model (and, recursively, its direct replies)
// into a response DTO using the pre-resolved lookups. It performs no database
// access. Reply nesting stays one level deep: only the top-level comment's
// Replies slice is walked.
func (m *commentResponseMapper) mapComment(c *models.VideoComment) response.CommentResponse {
	author := response.AuthorInfo{}

	if c.ClientID != nil {
		author.ID = *c.ClientID
		if cl, ok := m.clients[*c.ClientID]; ok && cl != nil {
			author.Name = cl.FirstName + " " + cl.LastName
			author.Avatar = cl.AvatarURL
		}
	} else if c.IsOfficial {
		// Official reply: author is the center that owns the feed post.
		author.IsCenter = true
		if fp, ok := m.posts[c.FeedPostID]; ok && fp != nil {
			author.CenterID = fp.BeautyCenterID
			// The center name will be filled by the controller layer if needed.
			author.Name = "Centro"
		}
	}

	// Recursively map replies.
	replies := make([]response.CommentResponse, 0, len(c.Replies))
	for i := range c.Replies {
		replies = append(replies, m.mapComment(&c.Replies[i]))
	}

	return response.CommentResponse{
		ID:         c.ID,
		Body:       c.Body,
		IsOfficial: c.IsOfficial,
		IsHidden:   c.IsHidden,
		CreatedAt:  c.CreatedAt,
		Author:     author,
		Replies:    replies,
	}
}
