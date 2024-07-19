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
