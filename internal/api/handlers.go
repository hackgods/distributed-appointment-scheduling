package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/hackgods/distributed-appointment-scheduling/internal/appointment"
	redisclient "github.com/hackgods/distributed-appointment-scheduling/internal/redis"
)

func createAppointmentHandler(svc *appointment.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateAppointmentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_body", "could not parse JSON")
			return
		}

		slotID, err := uuid.Parse(req.SlotID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_slot_id", "slot_id must be a valid UUID")
			return
		}

		patientID, err := uuid.Parse(req.PatientID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_patient_id", "patient_id must be a valid UUID")
			return
		}

		appt, err := svc.CreateAppointment(r.Context(), slotID, patientID)
		if err != nil {
			handleCreateError(w, err)
			return
		}

		resp := AppointmentResponse{
			ID:        appt.ID,
			SlotID:    appt.SlotID,
			PatientID: appt.PatientID,
			Status:    string(appt.Status),
			ExpiresAt: appt.ExpiresAt,
		}

		writeJSON(w, http.StatusCreated, resp)
	}
}

func confirmAppointmentHandler(svc *appointment.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		 idStr := chi.URLParam(r, "id")
		 id, err := uuid.Parse(idStr)
		 if err != nil {
			 writeError(w, http.StatusBadRequest, "invalid_appointment_id", "id must be a valid UUID")
			 return
		 }

		 appt, err := svc.ConfirmAppointment(r.Context(), id)
		 if err != nil {
			 handleConfirmError(w, err)
			 return
		 }

		 resp := AppointmentResponse{
			 ID:        appt.ID,
			 SlotID:    appt.SlotID,
			 PatientID: appt.PatientID,
			 Status:    string(appt.Status),
			 ExpiresAt: appt.ExpiresAt,
		 }

		 writeJSON(w, http.StatusOK, resp)
	}
}

func handleCreateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, appointment.ErrPatientNotFound):
		writeError(w, http.StatusNotFound, "patient_not_found", err.Error())
	case errors.Is(err, appointment.ErrSlotNotFound):
		writeError(w, http.StatusNotFound, "slot_not_found", err.Error())
	case errors.Is(err, appointment.ErrSlotNotOpen):
		writeError(w, http.StatusConflict, "slot_not_open", err.Error())
	case errors.Is(err, appointment.ErrSlotAlreadyBooked):
		writeError(w, http.StatusConflict, "slot_already_booked", err.Error())
	case errors.Is(err, appointment.ErrSlotBeingBooked),
		errors.Is(err, redisclient.ErrLockNotAcquired):
		writeError(w, http.StatusConflict, "slot_being_booked", "slot is currently being booked, please retry shortly")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

func handleConfirmError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, appointment.ErrAppointmentNotFound):
		writeError(w, http.StatusNotFound, "appointment_not_found", err.Error())
	case errors.Is(err, appointment.ErrAppointmentExpiredState):
		writeError(w, http.StatusConflict, "appointment_expired", err.Error())
	case errors.Is(err, appointment.ErrInvalidStatusTransition):
		writeError(w, http.StatusConflict, "invalid_status_transition", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}
