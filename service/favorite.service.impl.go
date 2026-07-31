package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
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
		return errors.New("already marked as favorite")
	}

	favorite := &models.Favorite{
		ClientID:       req.ClientID,
		BeautyCenterID: req.BeautyCenterID,
	}
	return s.FavoriteRepo.Create(favorite)
}

func (s *FavoriteServiceImpl) Delete(id uint) error {
	return s.FavoriteRepo.Delete(id)
}

func (s *FavoriteServiceImpl) FindByClientID(clientID uint) ([]response.FavoriteResponse, error) {
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
