package request

type CreateCommentRequest struct {
	Body     string `json:"body" validate:"required,max=1000"`
	ParentID *uint  `json:"parent_id,omitempty"`
}

type UpdateCommentRequest struct {
	Body string `json:"body" validate:"max=1000"`
}

type ReplyRequest struct {
	Body string `json:"body" validate:"required,max=1000"`
}