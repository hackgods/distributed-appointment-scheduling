package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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

func NewRouter(svc *appointment.Service) http.Handler {
	r := chi.NewRouter()

	r.Post("/appointments", createAppointmentHandler(svc))
	r.Post("/appointments/{id}/confirm", confirmAppointmentHandler(svc))

	return r
}
