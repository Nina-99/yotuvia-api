package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
)

type MembershipServiceImpl struct {
	MembershipRepository repository.MembershipRepository
}

func NewMembershipService(membershipRepo repository.MembershipRepository) MembershipService {
	return &MembershipServiceImpl{
		MembershipRepository: membershipRepo,
	}
}

func (s *MembershipServiceImpl) Create(req request.CreateMembershipRequest) error {
	membership := models.Membership{
		Name:           req.Name,
		Description:    req.Description,
		BeautyCenterID: req.BeautyCenterID,
		IsActive:       req.IsActive,
	}
	return s.MembershipRepository.Create(&membership)
}

func (s *MembershipServiceImpl) Update(id uint, req request.UpdateMembershipRequest) error {
	membership, err := s.MembershipRepository.FindByID(id)
	if err != nil {
		return err
	}

	if req.Name != "" {
		membership.Name = req.Name
	}
	if req.Description != "" {
		membership.Description = req.Description
	}
	if req.IsActive != nil {
		membership.IsActive = *req.IsActive
	}

	return s.MembershipRepository.Update(membership)
}

func (s *MembershipServiceImpl) Delete(id uint) error {
	return s.MembershipRepository.Delete(id)
}

func (s *MembershipServiceImpl) FindByID(id uint) (response.MembershipResponse, error) {
	membership, err := s.MembershipRepository.FindByID(id)
	if err != nil {
		return response.MembershipResponse{}, err
	}

	return response.MembershipResponse{
		ID:             membership.ID,
		Name:           membership.Name,
		Description:    membership.Description,
		BeautyCenterID: membership.BeautyCenterID,
		IsActive:       membership.IsActive,
	}, nil
}

func (s *MembershipServiceImpl) FindAll() ([]response.MembershipResponse, error) {
	memberships, err := s.MembershipRepository.FindAll()
	if err != nil {
		return nil, err
	}

	var res []response.MembershipResponse
	for _, m := range memberships {
		res = append(res, response.MembershipResponse{
			ID:             m.ID,
			Name:           m.Name,
			Description:    m.Description,
			BeautyCenterID: m.BeautyCenterID,
			IsActive:       m.IsActive,
		})
	}
	return res, nil
}
