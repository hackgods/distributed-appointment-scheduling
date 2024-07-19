-- Indexes for read-side queries

-- Index for listing appointments by patient
CREATE INDEX IF NOT EXISTS idx_appointments_patient_id_created_at
    ON appointments (patient_id, created_at DESC);

-- Index for listing appointments by slot
CREATE INDEX IF NOT EXISTS idx_appointments_slot_id_created_at
    ON appointments (slot_id, created_at DESC);

-- Composite index for patient queries with status filtering
CREATE INDEX IF NOT EXISTS idx_appointments_patient_status_created_at
    ON appointments (patient_id, status, created_at DESC);

