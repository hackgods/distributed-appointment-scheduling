package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/hackgods/distributed-appointment-scheduling/internal/appointment"
)

type AppointmentService interface {
	CreateAppointment(ctx Context, slotID, patientID uuid.UUID) (*appointment.Appointment, error)
	ConfirmAppointment(ctx Context, id uuid.UUID) (*appointment.Appointment, error)
}

type Context = interface {
	Done() <-chan struct{}
	Err() error
}

type RouterConfig struct {
	Service  *appointment.Service
	PgPool   *pgxpool.Pool
	Redis    *redis.Client
	Env      string
	Version  string
}

func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	// Apply middleware
	r.Use(RequestIDMiddleware)
	r.Use(LoggingMiddleware)

	// Health endpoints
	health := NewHealthHandler(cfg.PgPool, cfg.Redis, cfg.Env, cfg.Version)
	r.Get("/health/live", health.Liveness)
	r.Get("/health/ready", health.Readiness)

	// Appointment endpoints
	r.Post("/appointments", createAppointmentHandler(cfg.Service))
	r.Get("/appointments", listAppointmentsHandler(cfg.Service))
	r.Get("/appointments/{id}", getAppointmentHandler(cfg.Service))
	r.Post("/appointments/{id}/confirm", confirmAppointmentHandler(cfg.Service))

	return r
}
