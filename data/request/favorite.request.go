package request

type CreateFavoriteRequest struct {
	ClientID       uint `validate:"required"`
	BeautyCenterID uint `json:"beauty_center_id" validate:"required"`
}
