package api

import (
	"time"

	"github.com/google/uuid"
)

type CreateAppointmentRequest struct {
	SlotID    string `json:"slot_id"`
	PatientID string `json:"patient_id"`
}

type AppointmentResponse struct {
	ID        uuid.UUID  `json:"id"`
	SlotID    uuid.UUID  `json:"slot_id"`
	PatientID uuid.UUID  `json:"patient_id"`
	Status    string     `json:"status"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type AppointmentDetailResponse struct {
	ID        uuid.UUID  `json:"id"`
	Status    string      `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	ExpiresAt *time.Time  `json:"expires_at,omitempty"`

	Slot struct {
		ID        uuid.UUID  `json:"id"`
		StartTime time.Time  `json:"start_time"`
		EndTime   time.Time  `json:"end_time"`
		Status    string     `json:"status"`
		Capacity  int        `json:"capacity"`
	} `json:"slot"`

	Patient struct {
		ID    uuid.UUID `json:"id"`
		Name  string    `json:"name"`
		Email *string   `json:"email,omitempty"`
	} `json:"patient"`

	Clinician struct {
		ID        uuid.UUID `json:"id"`
		Name      string    `json:"name"`
		Specialty *string   `json:"specialty,omitempty"`
	} `json:"clinician"`
}

type AppointmentListResponse struct {
	Appointments []AppointmentDetailResponse `json:"appointments"`
	Total        int                         `json:"total,omitempty"`
}
