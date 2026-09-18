package main

import (
	"marketplace/gateway/internal/auth"
	"marketplace/gateway/internal/healthz"
	"marketplace/gateway/internal/listings"
	"marketplace/gateway/pkg/jwt"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-playground/validator"
)

func main() {
	r := chi.NewRouter()
	validate := validator.New()
	jwtManager := jwt.NewManager("secret-word", 15 * time.Minute, 7 * 24 * time.Hour)

	healthz.NewHealthzHandler(r)

	authService := auth.NewAuthService()
	auth.NewAuthHandler(r, *validate, authService)

	listingsService := listings.NewListingsService()
	listings.NewListingsHandler(r, jwtManager, listingsService)

	server := http.Server{
		Addr: ":8081",
		Handler: r,
	}
	server.ListenAndServe()
}