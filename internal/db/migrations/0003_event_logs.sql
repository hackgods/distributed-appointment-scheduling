-- Event log for audit trail

CREATE TABLE IF NOT EXISTS event_logs (
    id             bigserial PRIMARY KEY,
    event_type     text NOT NULL,
    appointment_id uuid REFERENCES appointments(id),
    payload        jsonb,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_event_logs_appointment_id
    ON event_logs (appointment_id);

CREATE INDEX IF NOT EXISTS idx_event_logs_event_type_created_at
    ON event_logs (event_type, created_at);
