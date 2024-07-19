package appointment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgRepository struct {
	pool *pgxpool.Pool
}

func NewPgRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool: pool}
}

// Helpers

func scanPatient(row pgx.Row) (*Patient, error) {
	var p Patient
	var email *string

	err := row.Scan(
		&p.ID,
		&p.Name,
		&email,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPatientNotFound
		}
		return nil, err
	}

	p.Email = email
	return &p, nil
}

func scanClinician(row pgx.Row) (*Clinician, error) {
	var c Clinician
	var specialty *string

	err := row.Scan(
		&c.ID,
		&c.Name,
		&specialty,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrClinicianNotFound
		}
		return nil, err
	}

	c.Specialty = specialty
	return &c, nil
}

func scanSlot(row pgx.Row) (*AppointmentSlot, error) {
	var s AppointmentSlot

	err := row.Scan(
		&s.ID,
		&s.PractitionerID,
		&s.StartTime,
		&s.EndTime,
		&s.Status,
		&s.Capacity,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSlotNotFound
		}
		return nil, err
	}

	return &s, nil
}

func scanAppointment(row pgx.Row) (*Appointment, error) {
	var a Appointment
	var expiresAt *time.Time

	err := row.Scan(
		&a.ID,
		&a.SlotID,
		&a.PatientID,
		&a.Status,
		&a.CreatedAt,
		&a.UpdatedAt,
		&expiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAppointmentNotFound
		}
		return nil, err
	}

	a.ExpiresAt = expiresAt
	return &a, nil
}

// Interface methods

func (r *PgRepository) GetPatientByID(ctx context.Context, id uuid.UUID) (*Patient, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, email, created_at, updated_at
		FROM patients
		WHERE id = $1
	`, id)
	return scanPatient(row)
}

func (r *PgRepository) GetClinicianByID(ctx context.Context, id uuid.UUID) (*Clinician, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, specialty, created_at, updated_at
		FROM clinicians
		WHERE id = $1
	`, id)
	return scanClinician(row)
}

func (r *PgRepository) GetSlotByID(ctx context.Context, id uuid.UUID) (*AppointmentSlot, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, practitioner_id, start_time, end_time, status, capacity, created_at, updated_at
		FROM appointment_slots
		WHERE id = $1
	`, id)
	return scanSlot(row)
}

func (r *PgRepository) GetAppointmentByID(ctx context.Context, id uuid.UUID) (*Appointment, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, slot_id, patient_id, status, created_at, updated_at, expires_at
		FROM appointments
		WHERE id = $1
	`, id)
	return scanAppointment(row)
}

func (r *PgRepository) GetConfirmedAppointmentForSlot(ctx context.Context, slotID uuid.UUID) (*Appointment, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, slot_id, patient_id, status, created_at, updated_at, expires_at
		FROM appointments
		WHERE slot_id = $1 AND status = 'confirmed'
	`, slotID)
	return scanAppointment(row)
}

func (r *PgRepository) CreatePendingAppointment(ctx context.Context, slotID, patientID uuid.UUID, expiresAt time.Time) (*Appointment, error) {
	id := uuid.New()

	row := r.pool.QueryRow(ctx, `
		INSERT INTO appointments (id, slot_id, patient_id, status, created_at, updated_at, expires_at)
		VALUES ($1, $2, $3, 'pending', now(), now(), $4)
		RETURNING id, slot_id, patient_id, status, created_at, updated_at, expires_at
	`, id, slotID, patientID, expiresAt)

	return scanAppointment(row)
}

func (r *PgRepository) UpdateAppointmentStatus(ctx context.Context, id uuid.UUID, from, to AppointmentStatus) (*Appointment, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE appointments
		SET status = $2,
		    updated_at = now()
		WHERE id = $1
		  AND status = $3
		RETURNING id, slot_id, patient_id, status, created_at, updated_at, expires_at
	`, id, to, from)

	return scanAppointment(row)
}

func (r *PgRepository) FindExpiredPending(ctx context.Context, now time.Time) ([]Appointment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, slot_id, patient_id, status, created_at, updated_at, expires_at
		FROM appointments
		WHERE status = 'pending'
		  AND expires_at IS NOT NULL
		  AND expires_at < $1
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Appointment
	for rows.Next() {
		a, err := scanAppointment(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *a)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *PgRepository) InsertEvent(ctx context.Context, ev EventLog) error {
	var appID *uuid.UUID
	if ev.AppointmentID != nil {
		appID = ev.AppointmentID
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO event_logs (event_type, appointment_id, payload, created_at)
		VALUES ($1, $2, $3, COALESCE($4, now()))
	`, ev.EventType, appID, ev.Payload, nullableTime(ev.CreatedAt))
	if err != nil {
		return fmt.Errorf("insert event log: %w", err)
	}

	return nil
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
