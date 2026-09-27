package repository

import "github.com/Nina-99/yottu-api/models"

type PersonnelRepository interface {
	Create(personnel *models.Personnel) error
	Update(personnel *models.Personnel) error
	Delete(id uint) error
	FindByID(id uint) (*models.Personnel, error)
	// FindAll returns a page of a centre's personnel. When serviceIDs is empty
	// the behaviour is unchanged (all personnel of the centre). When non-empty
	// it keeps only personnel who either have NO capability rows (unrestricted)
	// or whose rows cover ALL requested services — the SQL mirror of
	// service.PersonnelCoversServices.
	FindAll(beautyCenterID uint, serviceIDs []uint, page, limit int) ([]models.Personnel, int64, error)
	FindByEmail(email string) (*models.Personnel, error)
	// FindExistingIDs returns the subset of ids that resolve to a personnel row
	// in ONE query. It backs batched existence validation (e.g. the multi-
	// professional availability chain) without a per-id round trip.
	FindExistingIDs(ids []uint) (map[uint]bool, error)
}
