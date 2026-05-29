package request

type CreateBeautyCenterRequest struct {
	Name        string                     `json:"name" validate:"required,omitempty,min=5"`
	Address     string                     `json:"address" validate:"required,omitempty,min=5"`
	Email       string                     `json:"email" validate:"required,email,max=70"`
	Phone       string                     `json:"phone" validate:"required,max=20"`
	Url         string                     `json:"url"`
	Description string                     `json:"description" validate:"max=255"`
	Password    string                     `json:"password" validate:"required,min=6"`
	Images      []BeautyCenterImageRequest `json:"images"`
	Schedules   []WorkingHourRequest       `json:"schedules"`
}

type UpdateBeautyCenterRequest struct {
	Name        string                     `json:"name" validate:"omitempty,min=5"`
	Address     string                     `json:"address" validate:"max=omitempty,min=5"`
	Email       string                     `json:"email" validate:"required,email,max=70"`
	Phone       string                     `json:"phone" validate:"max=20"`
	Url         string                     `json:"url"`
	Description string                     `json:"description" validate:"max=255"`
	Password    string                     `json:"password" validate:"required"`
	IsActive    *bool                      `json:"is_active"`
	Images      []BeautyCenterImageRequest `json:"images"`
	Schedules   []WorkingHourRequest       `json:"schedules"`
}

type WorkingHourRequest struct {
	DayOfWeek int    `json:"day_of_week" validate:"required,min=0,max=6"`
	StartTime string `json:"start_time" validate:"required,len=5"` // HH:MM
	EndTime   string `json:"end_time" validate:"required,len=5"`   // HH:MM
}
