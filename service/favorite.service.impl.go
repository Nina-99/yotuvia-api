package service

import (
	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/go-playground/validator/v10"
)

type FavoriteServiceImpl struct {
	FavoriteRepo repository.FavoriteRepository
	Validate     *validator.Validate
}

func NewFavoriteServiceImpl(repo repository.FavoriteRepository, validate *validator.Validate) FavoriteService {
	return &FavoriteServiceImpl{
		FavoriteRepo: repo,
		Validate:     validate,
	}
}

func (s *FavoriteServiceImpl) Create(req request.CreateFavoriteRequest) error {
	err := s.Validate.Struct(req)
	if err != nil {
		return err
	}

	isFav, err := s.FavoriteRepo.IsFavorite(req.ClientID, req.BeautyCenterID)
	if err != nil {
		return err
	}
	if isFav {
		return apperrors.ErrConflict
	}

	favorite := &models.Favorite{
		ClientID:       req.ClientID,
		BeautyCenterID: req.BeautyCenterID,
	}
	return s.FavoriteRepo.Create(favorite)
}

func (s *FavoriteServiceImpl) Delete(id uint, actorClientID uint) error {
	favorite, err := s.FavoriteRepo.FindByID(id)
	if err != nil {
		return err
	}
	// A missing row follows the codebase's not-found convention (404), not the
	// no-op 200 a bare GORM Delete would produce.
	if favorite == nil {
		return apperrors.ErrNotFound
	}
	// Ownership: only the favorite's own client may remove it.
	if favorite.ClientID != actorClientID {
		return apperrors.ErrForbidden
	}
	return s.FavoriteRepo.Delete(id)
}

func (s *FavoriteServiceImpl) FindByClientID(clientID uint, actorClientID uint) ([]response.FavoriteResponse, error) {
	// Ownership: a caller may only read its OWN favorites. Favorites are a
	// private client surface with no staff consumer, so a mismatch is ErrForbidden
	// (independent of role: the JWT user_id claim is polymorphic per role, so a
	// staff id could otherwise collide with a client id).
	if clientID != actorClientID {
		return nil, apperrors.ErrForbidden
	}

	favorites, err := s.FavoriteRepo.FindByClientID(clientID)
	if err != nil {
		return nil, err
	}

	var res []response.FavoriteResponse
	for _, f := range favorites {
		r := response.FavoriteResponse{
			ID:             f.ID,
			ClientID:       f.ClientID,
			BeautyCenterID: f.BeautyCenterID,
		}
		r.BeautyCenter.ID = f.BeautyCenter.ID
		r.BeautyCenter.Name = f.BeautyCenter.Name
		r.BeautyCenter.Address = f.BeautyCenter.Address
		r.BeautyCenter.Phone = f.BeautyCenter.Phone
		r.BeautyCenter.Description = f.BeautyCenter.Description
		res = append(res, r)
	}
	return res, nil
}

func (s *FavoriteServiceImpl) IsFavorite(clientID uint, beautyCenterID uint) (bool, error) {
	return s.FavoriteRepo.IsFavorite(clientID, beautyCenterID)
}
