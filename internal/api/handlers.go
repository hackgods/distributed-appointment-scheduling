package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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

func getAppointmentHandler(svc *appointment.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_appointment_id", "id must be a valid UUID")
			return
		}

		detail, err := svc.GetAppointment(r.Context(), id)
		if err != nil {
			handleGetError(w, err)
			return
		}

		resp := toAppointmentDetailResponse(detail)
		writeJSON(w, http.StatusOK, resp)
	}
}

func listAppointmentsHandler(svc *appointment.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse query parameters
		patientIDStr := r.URL.Query().Get("patient_id")
		slotIDStr := r.URL.Query().Get("slot_id")
		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")

		// Parse limit and offset
		limit := 20
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}
		if limit > 100 {
			limit = 100
		}

		offset := 0
		if offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}

		var appointments []appointment.AppointmentDetail
		var err error

		// Route to appropriate service method based on query params
		if patientIDStr != "" {
			patientID, parseErr := uuid.Parse(patientIDStr)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "invalid_patient_id", "patient_id must be a valid UUID")
				return
			}
			appointments, err = svc.ListAppointmentsByPatient(r.Context(), patientID, limit, offset)
		} else if slotIDStr != "" {
			slotID, parseErr := uuid.Parse(slotIDStr)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "invalid_slot_id", "slot_id must be a valid UUID")
				return
			}
			appointments, err = svc.ListAppointmentsBySlot(r.Context(), slotID)
		} else {
			writeError(w, http.StatusBadRequest, "missing_filter", "must provide either patient_id or slot_id query parameter")
			return
		}

		if err != nil {
			if errors.Is(err, appointment.ErrAppointmentNotFound) ||
				errors.Is(err, appointment.ErrPatientNotFound) ||
				errors.Is(err, appointment.ErrSlotNotFound) {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		resp := AppointmentListResponse{
			Appointments: make([]AppointmentDetailResponse, len(appointments)),
		}
		for i, appt := range appointments {
			resp.Appointments[i] = toAppointmentDetailResponse(&appt)
		}
		resp.Total = len(appointments)

		writeJSON(w, http.StatusOK, resp)
	}
}

func handleGetError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, appointment.ErrAppointmentNotFound):
		writeError(w, http.StatusNotFound, "appointment_not_found", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

func toAppointmentDetailResponse(detail *appointment.AppointmentDetail) AppointmentDetailResponse {
	resp := AppointmentDetailResponse{
		ID:        detail.ID,
		Status:    string(detail.Status),
		CreatedAt: detail.CreatedAt,
		UpdatedAt: detail.UpdatedAt,
		ExpiresAt: detail.ExpiresAt,
	}

	if detail.Slot != nil {
		resp.Slot.ID = detail.Slot.ID
		resp.Slot.StartTime = detail.Slot.StartTime
		resp.Slot.EndTime = detail.Slot.EndTime
		resp.Slot.Status = string(detail.Slot.Status)
		resp.Slot.Capacity = detail.Slot.Capacity
	}

	if detail.Patient != nil {
		resp.Patient.ID = detail.Patient.ID
		resp.Patient.Name = detail.Patient.Name
		resp.Patient.Email = detail.Patient.Email
	}

	if detail.Clinician != nil {
		resp.Clinician.ID = detail.Clinician.ID
		resp.Clinician.Name = detail.Clinician.Name
		resp.Clinician.Specialty = detail.Clinician.Specialty
	}

	return resp
}
