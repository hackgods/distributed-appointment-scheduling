package appointment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/hackgods/distributed-appointment-scheduling/internal/config"
	redisclient "github.com/hackgods/distributed-appointment-scheduling/internal/redis"
)

const (
	EventAppointmentCreated   = "APPOINTMENT_CREATED"
	EventAppointmentConfirmed = "APPOINTMENT_CONFIRMED"
	EventAppointmentExpired   = "APPOINTMENT_EXPIRED"
)

var (
	ErrSlotAlreadyBooked       = errors.New("slot already has a confirmed appointment")
	ErrSlotBeingBooked         = errors.New("slot is currently being booked, please retry")
	ErrAppointmentExpiredState = errors.New("appointment is already expired")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	ErrSlotNotOpen             = errors.New("slot is not open")
)

type Service struct {
	repo   Repository
	locker redisclient.Locker
	cfg    config.Config
}

func NewService(repo Repository, locker redisclient.Locker, cfg config.Config) *Service {
	return &Service{
		repo:   repo,
		locker: locker,
		cfg:    cfg,
	}
}

// CreateAppointment tries to reserve a slot for a patient.
// It uses a distributed lock so that concurrent requests for the same slot
// cannot both create a pending appointment.
func (s *Service) CreateAppointment(ctx context.Context, slotID, patientID uuid.UUID) (*Appointment, error) {
	// Validate patient exists
	if _, err := s.repo.GetPatientByID(ctx, patientID); err != nil {
		if errors.Is(err, ErrPatientNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("load patient: %w", err)
	}

	// Validate slot exists and is open
	slot, err := s.repo.GetSlotByID(ctx, slotID)
	if err != nil {
		return nil, fmt.Errorf("load slot: %w", err)
	}
	if slot.Status != SlotOpen {
		return nil, ErrSlotNotOpen
	}

	var created *Appointment

	err = s.locker.WithSlotLock(ctx, slotID, func(lockCtx context.Context) error {
		// Inside the critical section re-check for confirmed appointment for this slot
		existing, err := s.repo.GetConfirmedAppointmentForSlot(lockCtx, slotID)
		if err != nil && !errors.Is(err, ErrAppointmentNotFound) {
			return fmt.Errorf("check confirmed appointment: %w", err)
		}
		if existing != nil {
			return ErrSlotAlreadyBooked
		}

		expiresAt := time.Now().Add(s.cfg.AppointmentTTL)
		appt, err := s.repo.CreatePendingAppointment(lockCtx, slotID, patientID, expiresAt)
		if err != nil {
			return fmt.Errorf("create pending appointment: %w", err)
		}

		created = appt

		payload := map[string]any{
			"slot_id":    slotID.String(),
			"patient_id": patientID.String(),
			"expires_at": expiresAt,
		}
		s.logEvent(lockCtx, appt.ID, EventAppointmentCreated, payload)

		return nil
	})

	if err != nil {
		if errors.Is(err, redisclient.ErrLockNotAcquired) {
			return nil, ErrSlotBeingBooked
		}
		if errors.Is(err, ErrSlotAlreadyBooked) {
			return nil, err
		}
		return nil, err
	}

	return created, nil
}

// ConfirmAppointment moves a pending appointment to confirmed
func (s *Service) ConfirmAppointment(ctx context.Context, id uuid.UUID) (*Appointment, error) {
	appt, err := s.repo.GetAppointmentByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load appointment: %w", err)
	}

	now := time.Now()

	if appt.Status == StatusExpired {
		return nil, ErrAppointmentExpiredState
	}

	if appt.ExpiresAt != nil && appt.ExpiresAt.Before(now) {
		// Try to mark it as expired if still pending
		_, updErr := s.repo.UpdateAppointmentStatus(ctx, appt.ID, StatusPending, StatusExpired)
		if updErr != nil && !errors.Is(updErr, ErrAppointmentNotFound) {
			log.Printf("failed to mark appointment %s as expired during confirm: %v", appt.ID, updErr)
		}
		s.logEvent(ctx, appt.ID, EventAppointmentExpired, map[string]any{
			"reason": "confirm_after_expiry",
		})
		return nil, ErrAppointmentExpiredState
	}

	if appt.Status != StatusPending {
		return nil, ErrInvalidStatusTransition
	}

	updated, err := s.repo.UpdateAppointmentStatus(ctx, appt.ID, StatusPending, StatusConfirmed)
	if err != nil {
		return nil, fmt.Errorf("confirm appointment: %w", err)
	}

	s.logEvent(ctx, updated.ID, EventAppointmentConfirmed, map[string]any{})

	return updated, nil
}

// ExpirePendingAppointments is intended to be called by the worker periodically
func (s *Service) ExpirePendingAppointments(ctx context.Context) error {
	now := time.Now()
	expiredCandidates, err := s.repo.FindExpiredPending(ctx, now)
	if err != nil {
		return fmt.Errorf("find expired pending appointments: %w", err)
	}

	for _, appt := range expiredCandidates {
		_, err := s.repo.UpdateAppointmentStatus(ctx, appt.ID, StatusPending, StatusExpired)
		if err != nil && !errors.Is(err, ErrAppointmentNotFound) {
			log.Printf("failed to expire appointment %s: %v", appt.ID, err)
			continue
		}
		s.logEvent(ctx, appt.ID, EventAppointmentExpired, map[string]any{
			"reason": "worker",
		})
	}

	return nil
}

func (s *Service) logEvent(ctx context.Context, appointmentID uuid.UUID, eventType string, payload map[string]any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to marshal event payload for %s: %v", eventType, err)
		data = nil
	}

	apptID := appointmentID

	ev := EventLog{
		EventType:     eventType,
		AppointmentID: &apptID,
		Payload:       data,
		CreatedAt:     time.Now(),
	}

	if err := s.repo.InsertEvent(ctx, ev); err != nil {
		log.Printf("failed to insert event log %s for appointment %s: %v", eventType, appointmentID, err)
	}
}
