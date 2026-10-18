package request

// UpsertVisitNoteRequest is the body of PUT /reservation/me/:id/note.
//
// The author is the caller (from the JWT) and the visit is the path id, so the
// body carries ONLY the text. An empty body is rejected here: clearing a note
// is the DELETE endpoint's job, so "blank note" and "no note" never blur.
type UpsertVisitNoteRequest struct {
	Body string `json:"body" binding:"required,max=2000"`
}
