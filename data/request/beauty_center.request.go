package request

type CreateBeautyCenterRequest struct {
	Currency    string                     `json:"currency" validate:"omitempty,len=3"`
	Name        string                     `json:"name" validate:"required,min=5"`
	Address     string                     `json:"address" validate:"required,min=5"`
	Email       string                     `json:"email" validate:"required,email,max=70"`
	Phone       string                     `json:"phone" validate:"required,max=20"`
	Url         string                     `json:"url"`
	Description string                     `json:"description" validate:"max=255"`
	NameI18n        map[string]string `json:"name_i18n"`
	DescriptionI18n map[string]string `json:"description_i18n"`
	Password    string                     `json:"password" validate:"required,min=8"`
	Images      []BeautyCenterImageRequest `json:"images"`
	Schedules   []WorkingHourRequest       `json:"schedules"`
}

type UpdateBeautyCenterRequest struct {
	Currency    string                     `json:"currency" validate:"omitempty,len=3"`
	Name        string                     `json:"name" validate:"omitempty"`
	Address     string                     `json:"address" validate:"omitempty,min=5"`
	Email       string                     `json:"email" validate:"omitempty,email,max=70"`
	Phone       string                     `json:"phone" validate:"omitempty,max=20"`
	Url         string                     `json:"url"`
	Description string                     `json:"description" validate:"omitempty,max=255"`
	NameI18n        map[string]string `json:"name_i18n"`
	DescriptionI18n map[string]string `json:"description_i18n"`
	Password    string                     `json:"password" validate:"omitempty,min=8"`
	Latitude    *float64                   `json:"latitude" validate:"omitempty"`
	Longitude   *float64                   `json:"longitude" validate:"omitempty"`
	IsActive    *bool                      `json:"is_active"`
	Images      []BeautyCenterImageRequest `json:"images"`
	Schedules   []WorkingHourRequest       `json:"schedules"`
	// Deposit ("seña") settings — pointers so false/0 are settable.
	DepositRequired       *bool    `json:"deposit_required"`
	DepositAmount         *float64 `json:"deposit_amount"`
	DepositPercent        *float64 `json:"deposit_percent"`
	DepositTrustThreshold *int     `json:"deposit_trust_threshold"`
	DepositHoldMinutes    *int     `json:"deposit_hold_minutes"`
	DepositQRURL          *string  `json:"deposit_qr_url"`
}

type WorkingHourRequest struct {
	DayOfWeek int    `json:"day_of_week" validate:"required,min=0,max=6"`
	StartTime string `json:"start_time" validate:"required,len=5"` // HH:MM
	EndTime   string `json:"end_time" validate:"required,len=5"`   // HH:MM
}

type SearchParams struct {
	Query      string   `json:"q"`
	Lat        float64  `json:"lat"`
	Lng        float64  `json:"lng"`
	Radius     float64  `json:"radius"`
	CategoryID *uint    `json:"category_id"`
	ServiceID  *uint    `json:"service_id"`
	PriceMin   *float64 `json:"price_min"`
	PriceMax   *float64 `json:"price_max"`
	Page       int      `json:"page"`
	Limit      int      `json:"limit"`
}
