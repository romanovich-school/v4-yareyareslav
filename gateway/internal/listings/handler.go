package listings

import (
	"context"
	"marketplace/gateway/config"
	"marketplace/gateway/internal/auth"
	"marketplace/gateway/pkg/response"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/go-playground/validator"
)

type Service interface {
	FindAll(ctx context.Context) ([]Listing, error)
	FindOne(ctx context.Context, id int) (Listing, error)
	CreateOne(ctx context.Context) (Listing, error)
	PatchOne(ctx context.Context) (Listing, error)
	DeleteOne(ctx context.Context, id int) (Listing, error)
}

type ListingsHandler struct {
	validate *validator.Validate
	service  Service
}

func NewListingsHandler(
	r chi.Router,
	validate *validator.Validate,
	ac config.AuthConfig,
	service Service,
) *ListingsHandler {
	h := &ListingsHandler{
		validate: validate,
		service:  service,
	}

	lr := chi.NewRouter()
	lr.Get("/", h.GetAllListings)
	lr.Get("/{id}", h.GetOneListing)
	lr.Post("/", ac.Middleware.CheckUser(h.PostListing(), auth.Seller))
	lr.Patch("/{id}", ac.Middleware.CheckUser(h.PatchListing(), auth.Seller))
	lr.Delete("/{id}", ac.Middleware.CheckUser(h.DeleteListing(), auth.Seller))

	r.Mount("/listings", lr)

	return h
}

func (h *ListingsHandler) GetAllListings(w http.ResponseWriter, r *http.Request) {
	listings, err := h.service.FindAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := response.Body(w, listings, http.StatusOK); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *ListingsHandler) GetOneListing(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	listing, err := h.service.FindOne(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := response.Body(w, listing, http.StatusOK); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *ListingsHandler) PostListing() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		listing, err := h.service.CreateOne(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := response.Body(w, listing, http.StatusCreated); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (h *ListingsHandler) PatchListing() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		listing, err := h.service.PatchOne(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		if err := response.Body(w, listing, http.StatusOK); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (h *ListingsHandler) DeleteListing() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idParam := chi.URLParam(r, "id")
		id, err := strconv.Atoi(idParam)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		listing, err := h.service.DeleteOne(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := response.Body(w, listing, http.StatusNoContent); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
