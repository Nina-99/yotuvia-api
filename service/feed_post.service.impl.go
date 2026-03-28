package service

import (
	"errors"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/go-playground/validator/v10"
)

type FeedPostServiceImpl struct {
	FeedPostRepository     repository.FeedPostRepository
	BeautyCenterRepository repository.BeautyCenterRepository
	Validate               *validator.Validate
}

func NewFeedPostServiceImpl(
	feedPostRepo repository.FeedPostRepository,
	beautyCenterRepo repository.BeautyCenterRepository,
	validate *validator.Validate,
) FeedPostService {
	return &FeedPostServiceImpl{
		FeedPostRepository:     feedPostRepo,
		BeautyCenterRepository: beautyCenterRepo,
		Validate:               validate,
	}
}

func (s *FeedPostServiceImpl) Create(req request.CreateFeedPostRequest) (response.FeedPostResponse, error) {
	if err := s.Validate.Struct(req); err != nil {
		return response.FeedPostResponse{}, err
	}

	// Validate BeautyCenter exists
	center, err := s.BeautyCenterRepository.FindByID(req.BeautyCenterID)
	if err != nil || center == nil {
		return response.FeedPostResponse{}, errors.New("beauty center not found")
	}

	post := models.FeedPost{
		BeautyCenterID: req.BeautyCenterID,
		PersonnelID:    req.PersonnelID,
		ServiceID:      req.ServiceID,
		VideoURL:       req.VideoURL,
		ThumbnailURL:   req.ThumbnailURL,
		Caption:        req.Caption,
		Duration:       req.Duration,
	}

	if err := s.FeedPostRepository.Create(&post); err != nil {
		return response.FeedPostResponse{}, err
	}

	return s.mapToResponse(post), nil
}

func (s *FeedPostServiceImpl) Update(id uint, req request.UpdateFeedPostRequest) (response.FeedPostResponse, error) {
	if err := s.Validate.Struct(req); err != nil {
		return response.FeedPostResponse{}, err
	}

	post, err := s.FeedPostRepository.FindByID(id)
	if err != nil {
		return response.FeedPostResponse{}, err
	}

	if req.Caption != "" {
		post.Caption = req.Caption
	}
	if req.VideoURL != "" {
		post.VideoURL = req.VideoURL
	}
	if req.ThumbnailURL != "" {
		post.ThumbnailURL = req.ThumbnailURL
	}

	if err := s.FeedPostRepository.Update(post); err != nil {
		return response.FeedPostResponse{}, err
	}

	return s.mapToResponse(*post), nil
}

func (s *FeedPostServiceImpl) Delete(id uint) error {
	return s.FeedPostRepository.Delete(id)
}

func (s *FeedPostServiceImpl) FindByID(id uint) (response.FeedPostResponse, error) {
	post, err := s.FeedPostRepository.FindByID(id)
	if err != nil {
		return response.FeedPostResponse{}, err
	}
	return s.mapToResponse(*post), nil
}

func (s *FeedPostServiceImpl) FindAll() ([]response.FeedPostResponse, error) {
	posts, err := s.FeedPostRepository.FindAll()
	if err != nil {
		return nil, err
	}

	var res []response.FeedPostResponse
	for _, post := range posts {
		res = append(res, s.mapToResponse(post))
	}
	return res, nil
}

func (s *FeedPostServiceImpl) FindByBeautyCenterID(centerID uint) ([]response.FeedPostResponse, error) {
	posts, err := s.FeedPostRepository.FindByBeautyCenterID(centerID)
	if err != nil {
		return nil, err
	}

	var res []response.FeedPostResponse
	for _, post := range posts {
		res = append(res, s.mapToResponse(post))
	}
	return res, nil
}

func (s *FeedPostServiceImpl) mapToResponse(post models.FeedPost) response.FeedPostResponse {
	return response.FeedPostResponse{
		ID:             post.ID,
		BeautyCenterID: post.BeautyCenterID,
		BeautyCenter: response.BeautyCenterResponse{
			ID:      post.BeautyCenter.ID,
			Name:    post.BeautyCenter.Name,
			Address: post.BeautyCenter.Address,
		},
		PersonnelID:  post.PersonnelID,
		ServiceID:    post.ServiceID,
		VideoURL:     post.VideoURL,
		ThumbnailURL: post.ThumbnailURL,
		Caption:      post.Caption,
		Duration:     post.Duration,
		ViewsCount:   post.ViewsCount,
		LikesCount:   post.LikesCount,
		CreatedAt:    post.CreatedAt,
	}
}
