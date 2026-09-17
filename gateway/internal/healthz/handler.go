package healthz

import (
	"net/http"

	"github.com/go-chi/chi"
)

type HealthzHandler struct {
}

func NewHealthzHandler(r chi.Router) *HealthzHandler {
	h := &HealthzHandler{}
	r.Get("/healthz", h.CheckHealthz)

	return h
}

func (h *HealthzHandler) CheckHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}