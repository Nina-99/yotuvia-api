package response

import "time"

type ReservationResponse struct {
	ID             uint      `json:"id"`
	BeautyCenterID uint      `json:"beauty_center_id"`
	ClientID       uint      `json:"client_id"`
	Date           time.Time `json:"date"`
	StartTime      string    `json:"start_time"`
	EndTime        string    `json:"end_time"`
	Status         string    `json:"status"`
}
