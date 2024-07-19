package appointment

import (
	"time"

	"github.com/google/uuid"
)

type AppointmentStatus string

const (
	StatusPending   AppointmentStatus = "pending"
	StatusConfirmed AppointmentStatus = "confirmed"
	StatusCancelled AppointmentStatus = "cancelled"
	StatusExpired   AppointmentStatus = "expired"
)

type SlotStatus string

const (
	SlotOpen    SlotStatus = "open"
	SlotBlocked SlotStatus = "blocked"
	SlotDeleted SlotStatus = "deleted"
)

type Patient struct {
	ID        uuid.UUID
	Name      string
	Email     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Clinician struct {
	ID        uuid.UUID
	Name      string
	Specialty *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AppointmentSlot struct {
	ID             uuid.UUID
	PractitionerID uuid.UUID
	StartTime      time.Time
	EndTime        time.Time
	Status         SlotStatus
	Capacity       int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Appointment struct {
	ID        uuid.UUID
	SlotID    uuid.UUID
	PatientID uuid.UUID
	Status    AppointmentStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt *time.Time
}

type EventLog struct {
	ID            int64
	EventType     string
	AppointmentID *uuid.UUID
	Payload       []byte
	CreatedAt     time.Time
}

type AppointmentDetail struct {
	Appointment
	Slot      *AppointmentSlot
	Patient   *Patient
	Clinician *Clinician
}
