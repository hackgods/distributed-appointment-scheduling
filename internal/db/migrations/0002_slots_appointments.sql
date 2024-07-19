-- Slot and appointment tables with constraints

CREATE TABLE IF NOT EXISTS appointment_slots (
    id               uuid PRIMARY KEY,
    practitioner_id  uuid NOT NULL REFERENCES clinicians(id),
    start_time       timestamptz NOT NULL,
    end_time         timestamptz NOT NULL,
    status           slot_status NOT NULL DEFAULT 'open',
    capacity         integer NOT NULL DEFAULT 1,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT chk_slot_time_range CHECK (end_time > start_time)
);

-- Prevent duplicate slots for the same practitioner and time range.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_slot_practitioner_time
    ON appointment_slots (practitioner_id, start_time, end_time);

CREATE TABLE IF NOT EXISTS appointments (
    id           uuid PRIMARY KEY,
    slot_id      uuid NOT NULL REFERENCES appointment_slots(id),
    patient_id   uuid NOT NULL REFERENCES patients(id),
    status       appointment_status NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz,

    CONSTRAINT chk_expires_future CHECK (expires_at IS NULL OR expires_at > created_at)
);

-- For expiring pending appointments efficiently
CREATE INDEX IF NOT EXISTS idx_appointments_status_expires_at
    ON appointments (status, expires_at);

-- DB-level invariant: only one confirmed appointment per slot.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_confirmed_appointment_per_slot
    ON appointments (slot_id)
    WHERE status = 'confirmed';
