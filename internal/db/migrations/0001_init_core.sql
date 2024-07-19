-- Core tables: patients and clinicians, plus enum types

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'appointment_status') THEN
        CREATE TYPE appointment_status AS ENUM ('pending', 'confirmed', 'cancelled', 'expired');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'slot_status') THEN
        CREATE TYPE slot_status AS ENUM ('open', 'blocked', 'deleted');
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS patients (
    id          uuid PRIMARY KEY,
    name        text NOT NULL,
    email       text UNIQUE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS clinicians (
    id          uuid PRIMARY KEY,
    name        text NOT NULL,
    specialty   text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
