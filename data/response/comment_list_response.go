package response

type CommentListResponse struct {
	Comments   []CommentResponse `json:"comments"`
	NextCursor *uint   `json:"next_cursor,omitempty"`
}