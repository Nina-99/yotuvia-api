package response

type BeautyCenterResponse struct {
	ID          uint                    `json:"id"`
	Name        string                  `json:"name"`
	Address     string                  `json:"address"`
	Email       string                  `json:"email"`
	Phone       string                  `json:"phone"`
	Url         string                  `json:"url"`
	Description string                  `json:"description"`
	IsActive    bool                    `json:"is_active"`
	Images      []BeautyCenterImageResp `json:"images"`
	Schedules   []WorkingHourResp       `json:"schedules"`
}

type BeautyCenterImageResp struct {
	ID       uint   `json:"id"`
	ImageUrl string `json:"image_url"`
	IsCover  bool   `json:"is_cover"`
	Position int    `json:"position"`
}

type WorkingHourResp struct {
	ID        uint   `json:"id"`
	DayOfWeek int    `json:"day_of_week"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}
