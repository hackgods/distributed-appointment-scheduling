package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hackgods/distributed-appointment-scheduling/internal/db"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("seed starting")

	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		log.Fatal("POSTGRES_DSN is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.ConnectPostgres(ctx, dsn)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	gofakeit.Seed(time.Now().UnixNano())

	if err := seedClinicians(context.Background(), pool, 100); err != nil {
		log.Fatalf("seed clinicians: %v", err)
	}
	if err := seedPatients(context.Background(), pool, 9000); err != nil {
		log.Fatalf("seed patients: %v", err)
	}

	log.Println("seed complete")
}

func seedClinicians(ctx context.Context, pool *pgxpool.Pool, count int) error {
	log.Printf("seeding %d clinicians", count)

	specialties := []string{
		"Dermatology",
		"Cardiology",
		"General Practice",
		"Orthopedics",
		"Endocrinology",
		"Neurology",
		"Pediatrics",
		"Psychiatry",
		"Ophthalmology",
		"ENT",
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for i := 0; i < count; i++ {
		id := uuid.New()
		name := gofakeit.Name()
		spec := specialties[gofakeit.Number(0, len(specialties)-1)]

		_, err := tx.Exec(ctx, `
			INSERT INTO clinicians (id, name, specialty, created_at, updated_at)
			VALUES ($1, $2, $3, now(), now())
		`, id, name, spec)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	log.Println("clinicians seeded")
	return nil
}

func seedPatients(ctx context.Context, pool *pgxpool.Pool, count int) error {
	log.Printf("seeding %d patients", count)

	const batchSize = 500

	for offset := 0; offset < count; offset += batchSize {
		end := offset + batchSize
		if end > count {
			end = count
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}

		for i := offset; i < end; i++ {
			id := uuid.New()
			name := gofakeit.Name()
			email := gofakeit.Email()

			_, err := tx.Exec(ctx, `
				INSERT INTO patients (id, name, email, created_at, updated_at)
				VALUES ($1, $2, $3, now(), now())
			`, id, name, email)
			if err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return err
		}

		log.Printf("patients seeded: %d/%d", end, count)
	}

	log.Println("patients seeded")
	return nil
}
