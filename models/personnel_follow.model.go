package models

import "gorm.io/gorm"

// PersonnelFollow records that a client follows a specific professional so the
// professional's new videos reach the client's feed/notifications. The
// composite UNIQUE index on (client_id, personnel_id) makes double-follow
// impossible at the DB layer, regardless of application races.
type PersonnelFollow struct {
	gorm.Model
	ClientID    uint `gorm:"not null;uniqueIndex:idx_personnel_follows_client_personnel" json:"client_id"`
	PersonnelID uint `gorm:"not null;uniqueIndex:idx_personnel_follows_client_personnel" json:"personnel_id"`

	Client    Client    `gorm:"foreignKey:ClientID" json:"-"`
	Personnel Personnel `gorm:"foreignKey:PersonnelID" json:"-"`
}
