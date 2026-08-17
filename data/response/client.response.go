package response

import "time"

type ClientResponse struct {
	ID        uint   `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	AvatarURL string `json:"avatar_url"`
	IsActive  bool   `json:"is_active"`
}

// AdminClientResponse is the beauty-center admin list DTO (design D2). It is
// kept separate from ClientResponse so the /client/me profile contract never
// changes. Metrics reflect COMPLETED reservations only; LastVisit is nil when
// the client has no completed visit at the center.
type AdminClientResponse struct {
	ID          uint       `json:"id"`
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	AvatarURL   string     `json:"avatar_url"`
	IsActive    bool       `json:"is_active"`
	VisitsCount int        `json:"visits_count"`
	TotalSpent  float64    `json:"total_spent"`
	LastVisit   *time.Time `json:"last_visit"`
}
