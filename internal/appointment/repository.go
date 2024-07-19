package appointment

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPatientNotFound     = errors.New("patient not found")
	ErrClinicianNotFound   = errors.New("clinician not found")
	ErrSlotNotFound        = errors.New("slot not found")
	ErrAppointmentNotFound = errors.New("appointment not found")
)

// Repository contains all DB interactions needed by the service.
type Repository interface {
	GetPatientByID(ctx context.Context, id uuid.UUID) (*Patient, error)
	GetClinicianByID(ctx context.Context, id uuid.UUID) (*Clinician, error)

	GetSlotByID(ctx context.Context, id uuid.UUID) (*AppointmentSlot, error)

	// For conflict checks
	GetConfirmedAppointmentForSlot(ctx context.Context, slotID uuid.UUID) (*Appointment, error)
	GetAppointmentByID(ctx context.Context, id uuid.UUID) (*Appointment, error)

	// Creation and updates
	CreatePendingAppointment(ctx context.Context, slotID, patientID uuid.UUID, expiresAt time.Time) (*Appointment, error)
	UpdateAppointmentStatus(ctx context.Context, id uuid.UUID, from, to AppointmentStatus) (*Appointment, error)

	// Expiry worker
	FindExpiredPending(ctx context.Context, now time.Time) ([]Appointment, error)

	// Event logging
	InsertEvent(ctx context.Context, ev EventLog) error
}
