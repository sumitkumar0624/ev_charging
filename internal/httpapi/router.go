package httpapi

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/example/evcharging/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Handler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewRouter(svc *service.Service, logger *slog.Logger) http.Handler {
	h := &Handler{service: svc, logger: logger}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(requestLogger(logger))
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Route("/v1", func(r chi.Router) {
		r.Post("/drivers", h.registerDriver)
		r.Get("/drivers/{driverID}/sessions", h.driverSessions)
		r.Post("/stations", h.registerStation)
		r.Get("/stations/{stationID}/sessions", h.stationSessions)
		r.Patch("/connectors/{connectorID}/status", h.setConnectorStatus)
		r.Post("/sessions", h.startSession)
		r.Post("/sessions/{sessionID}/end", h.endSession)
		r.Post("/promo-codes", h.createPromo)
		r.Delete("/promo-codes/{code}", h.deletePromo)
	})
	return r
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("request", "method", r.Method, "path", strings.TrimSpace(r.URL.Path),
				"request_id", middleware.GetReqID(r.Context()))
			next.ServeHTTP(w, r)
		})
	}
}
