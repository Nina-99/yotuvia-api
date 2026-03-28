package request

type CreateBeautyCenterRequest struct {
	Name        string                     `json:"name" binding:"required,max=50"`
	Address     string                     `json:"address" binding:"required,max=50"`
	Email       string                     `json:"email" binding:"required,email,max=70"`
	Phone       string                     `json:"phone" binding:"required,max=20"`
	Url         string                     `json:"url"`
	Description string                     `json:"description" binding:"max=255"`
	Password    string                     `json:"password" binding:"required,min=6"`
	Images      []BeautyCenterImageRequest `json:"images"`
	Schedules   []WorkingHourRequest       `json:"schedules"`
}

type UpdateBeautyCenterRequest struct {
	Name        string                     `json:"name" binding:"max=50"`
	Address     string                     `json:"address" binding:"max=50"`
	Phone       string                     `json:"phone" binding:"max=20"`
	Url         string                     `json:"url"`
	Description string                     `json:"description" binding:"max=255"`
	IsActive    *bool                      `json:"is_active"`
	Images      []BeautyCenterImageRequest `json:"images"`
	Schedules   []WorkingHourRequest       `json:"schedules"`
}

type BeautyCenterImageRequest struct {
	ImageUrl string `json:"image_url" binding:"required"`
	IsCover  bool   `json:"is_cover"`
	Position int    `json:"position"`
}

type WorkingHourRequest struct {
	DayOfWeek int    `json:"day_of_week" binding:"required,min=0,max=6"`
	StartTime string `json:"start_time" binding:"required,len=5"` // HH:MM
	EndTime   string `json:"end_time" binding:"required,len=5"`   // HH:MM
}
