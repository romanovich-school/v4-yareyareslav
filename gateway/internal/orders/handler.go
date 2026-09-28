package orders

import (
	"marketplace/gateway/config"
	"marketplace/gateway/internal/auth"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-playground/validator"
)

type Service interface {
	CreateOrders()
	FindOrders()
}

type OrdersHandler struct {
	r chi.Router
	validate *validator.Validate
	authConfig config.AuthConfig
	service Service
}

func NewOrdersHandler(
	r chi.Router, 
	validate *validator.Validate, 
	ac config.AuthConfig,
	service Service,
) *OrdersHandler {
	h := &OrdersHandler{}

	ordersRouter := chi.NewRouter()
	ordersRouter.Post("/", ac.Middleware.CheckUser(h.PostOrders(), auth.Buyer))
	ordersRouter.Get("/", ac.Middleware.CheckUser(h.GetOrders(), auth.Buyer))
	r.Mount("/orders", ordersRouter)

	return h
}

func (h *OrdersHandler) GetOrders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

	}
}

func (h *OrdersHandler) PostOrders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

	}
}