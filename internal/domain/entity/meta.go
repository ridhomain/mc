package entity

type SendMetaMessage struct {
	Type            string          `json:"type" binding:"required,oneof=text image document"`
	To              string          `json:"to" binding:"required"`
	Message         MetaMessage     `json:"message" binding:"required"`
	MetaCredentials MetaCredentials `json:"metaCredentials" binding:"required"`
	CompanyID       string          `json:"companyId" binding:"required"`
	ScheduleAt      *string         `json:"scheduleAt" binding:"omitempty,datetime"`
}

type MetaMessage struct {
	Text        *string `json:"text,omitempty"`
	ImageURL    *string `json:"imageUrl,omitempty"`
	DocumentURL *string `json:"documentUrl,omitempty"`
	Filename    *string `json:"filename,omitempty"`
}

type MetaCredentials struct {
	AccessToken   string `json:"accessToken" binding:"required"`
	PhoneNumberID string `json:"phoneNumberId" binding:"required"`
}

type SendMetaMessageResponse struct {
	Status     string                 `json:"status"`
	ScheduleAt *string                `json:"scheduleAt,omitempty"`
	Result     map[string]interface{} `json:"result,omitempty"`
}
