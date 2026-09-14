package main

import (
	"marketplace/gateway/internal/healthz"
	"net/http"

	"github.com/go-chi/chi"
)

func main() {
	

	r := chi.NewRouter()

	healthz.NewHealthzHandler(r)
	server := http.Server{
		Addr: ":8081",
		Handler: r,
	}
	server.ListenAndServe()
}