package service

import (
	"mime/multipart"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
)

type ClientService interface {
	// Update/Delete/FindByID are owner-only: the actorClientID claim must equal
	// the :id path parameter, enforced at the service entry point (defense in
	// depth — controllers pass the claim from ctx).
	Update(id uint, actorClientID uint, req request.UpdateClientRequest) error
	Delete(id uint, actorClientID uint) error
	FindByID(id uint, actorClientID uint) (*models.Client, error)
	FindAll(page, limit int) ([]models.Client, int64, error)
	FindEmail(email string) (*models.Client, error)
	// UploadAvatar stores an image under uploads/clients/{clientID}/ and returns
	// the relative /uploads/... URL. It writes the file only — persistence of
	// the returned URL happens later via Update(avatar_url).
	UploadAvatar(clientID uint, file *multipart.FileHeader) (string, error)
}
