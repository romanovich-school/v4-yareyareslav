package main

import (
	"marketplace/gateway/internal/auth"
	"marketplace/gateway/internal/healthz"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-playground/validator"
)

func main() {
	r := chi.NewRouter()
	validate := validator.New()

	healthz.NewHealthzHandler(r)

	authService := auth.NewAuthService()
	auth.NewAuthHandler(r, *validate, authService)

	server := http.Server{
		Addr: ":8081",
		Handler: r,
	}
	server.ListenAndServe()
}