package auth

import (
	"context"
	"encoding/json"
	"marketplace/gateway/pkg/request"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-playground/validator"
)

type Service interface {
	Register(ctx context.Context, r RegisterUserRequest) (RegisterUserResponse, error)
	Login(ctx context.Context, r LoginUserRequest) (LoginUserResponse, error)
	Refresh(ctx context.Context, r RefreshTokenRequest) (RefreshTokenResponse, error)
}

type AuthHandler struct {
	validate validator.Validate
	service Service
}

func NewAuthHandler(
	r chi.Router, 
	validate validator.Validate, 
	s Service,
) *AuthHandler {
	h := &AuthHandler{
		service: s,
		validate: validate,
	}

	authRouter := chi.NewRouter()
	authRouter.Post("/register", h.Register)
	authRouter.Post("/login", h.Login)
	authRouter.Post("/refresh", h.Refresh)

	r.Mount("/auth", authRouter)

	return h
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {	
	registerReq, err := request.Body[RegisterUserRequest](w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.validate.Struct(registerReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.service.Register(r.Context(), registerReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resultJson, err := json.Marshal(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} 

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(resultJson))

}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	loginReq, err := request.Body[LoginUserRequest](w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.validate.Struct(loginReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.service.Login(r.Context(), loginReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resultJson, err := json.Marshal(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} 

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(resultJson))
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshReq, err := request.Body[RefreshTokenRequest](w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.validate.Struct(refreshReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.service.Refresh(r.Context(), refreshReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resultJson, err := json.Marshal(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} 
	
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(resultJson))
}


