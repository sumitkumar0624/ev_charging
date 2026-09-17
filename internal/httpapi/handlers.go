package httpapi

import (
	"net/http"

	"github.com/example/evcharging/internal/models"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) registerDriver(w http.ResponseWriter, r *http.Request) {
	var input models.RegisterDriverInput
	if !h.decode(w, r, &input) {
		return
	}
	driver, err := h.service.RegisterDriver(r.Context(), input)
	h.respond(w, r, driver, err, http.StatusCreated)
}

func (h *Handler) registerStation(w http.ResponseWriter, r *http.Request) {
	var input models.RegisterStationInput
	if !h.decode(w, r, &input) {
		return
	}
	station, err := h.service.RegisterStation(r.Context(), input)
	h.respond(w, r, station, err, http.StatusCreated)
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request) {
	var input models.StartSessionInput
	if !h.decode(w, r, &input) {
		return
	}
	session, err := h.service.StartSession(r.Context(), input)
	h.respond(w, r, session, err, http.StatusCreated)
}

func (h *Handler) endSession(w http.ResponseWriter, r *http.Request) {
	var body models.EndSessionInput
	if !h.decode(w, r, &body) {
		return
	}
	session, err := h.service.EndSession(r.Context(), chi.URLParam(r, "sessionID"), body.EnergyDeliveredKWh)
	h.respond(w, r, session, err, http.StatusOK)
}

func (h *Handler) setConnectorStatus(w http.ResponseWriter, r *http.Request) {
	var body models.SetConnectorStatusInput
	if !h.decode(w, r, &body) {
		return
	}
	connector, err := h.service.SetConnectorStatus(r.Context(), chi.URLParam(r, "connectorID"), body.Status)
	h.respond(w, r, connector, err, http.StatusOK)
}

func (h *Handler) createPromo(w http.ResponseWriter, r *http.Request) {
	var input models.CreatePromoInput
	if !h.decode(w, r, &input) {
		return
	}
	promo, err := h.service.CreatePromo(r.Context(), input)
	h.respond(w, r, promo, err, http.StatusCreated)
}

func (h *Handler) deletePromo(w http.ResponseWriter, r *http.Request) {
	err := h.service.DeletePromo(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		h.respond(w, r, nil, err, 0)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) driverSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.service.DriverSessions(r.Context(), chi.URLParam(r, "driverID"))
	h.respond(w, r, sessions, err, http.StatusOK)
}

func (h *Handler) stationSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.service.StationSessions(r.Context(), chi.URLParam(r, "stationID"))
	h.respond(w, r, sessions, err, http.StatusOK)
}
