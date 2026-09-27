package service

import (
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/data/response"
)

type PersonnelService interface {
	Update(id uint, req request.UpdatePersonnelRequest) (response.PersonnelResponse, error)
	// UpdateMe is the self-service profile edit: the allow-listed DTO carries no
	// email/is_active/beauty_center_id, so identity and center affiliation are
	// structurally immutable through PATCH /personnel/me (design D2/D4).
	UpdateMe(id uint, req request.UpdateMePersonnelRequest) (response.PersonnelResponse, error)
	Delete(id uint) error
	FindByID(id uint) (response.PersonnelResponse, error)
	// FindAll returns a page of a centre's personnel. serviceIDs is the optional
	// booking-picker capability filter: empty means unchanged behaviour, while a
	// non-empty set keeps only personnel who are unrestricted or cover EVERY
	// requested service. An unknown service id is ErrBadRequest.
	FindAll(beautyCenterID uint, serviceIDs []uint, page, limit int) ([]response.PersonnelResponse, int64, error)
}
