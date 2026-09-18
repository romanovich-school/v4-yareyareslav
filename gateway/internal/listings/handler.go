package listings

import (
	"context"
	"marketplace/gateway/pkg/jwt"
	"marketplace/gateway/pkg/response"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
)

type Service interface {
	FindAll(ctx context.Context) ([]Listing, error)
	FindOne(ctx context.Context, id int) (Listing, error)
	CreateOne(ctx context.Context) (Listing, error)
	PatchOne(ctx context.Context) (Listing, error)
	DeleteOne(ctx context.Context, id int) (Listing, error)
}

type ListingsHandler struct {
	jwtManager *jwt.Manager
	service Service
}

func NewListingsHandler(r chi.Router, jwtManager *jwt.Manager, service Service) *ListingsHandler {
	h := &ListingsHandler{
		jwtManager: jwtManager,
		service: service,
	}

	lr := chi.NewRouter()
	lr.Get("/", h.GetAllListings)
	lr.Get("/{id}", h.GetOneListing)
	lr.Post("/", h.middlewareJWT(h.PostListing(), jwt.Seller))
	lr.Patch("/{id}", h.middlewareJWT(h.PatchListing(), jwt.Seller))
	lr.Delete("/{id}", h.middlewareJWT(h.DeleteListing(), jwt.Seller))

	r.Mount("/listings", lr)

	return h
}

func (h *ListingsHandler) middlewareJWT(next http.Handler, role jwt.Role) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			http.Error(w, "No authorization token", http.StatusForbidden)
			return
		}

		bearer := strings.Split(authorization, " ")
		if len(bearer) != 2 {
			http.Error(w, "Invalid authorization token", http.StatusBadRequest)
			return
		}

		claims, err := h.jwtManager.Parse(bearer[0])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if claims.Role != role {
			http.Error(w, "{error: Authorization provide no permission for the request}", http.StatusMethodNotAllowed)
			return
		}

		next.ServeHTTP(w, r)
	}
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