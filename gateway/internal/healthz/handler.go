package healthz

import (
	"context"
	"log"
	"log/slog"
	"marketplace/gateway/pkg/requestid"
	"marketplace/gateway/pkg/response"
	"marketplace/gen/catalogpb"
	"marketplace/gen/orderspb"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type Microservice struct {
	name    string
	service string
	client  healthpb.HealthClient
}

type HealthzHandler struct {
	catalog healthpb.HealthClient
	orders  healthpb.HealthClient
}

func NewHealthzHandler(r chi.Router, catalog, orders healthpb.HealthClient) *HealthzHandler {
	h := &HealthzHandler{catalog: catalog, orders: orders}
	r.Get("/healthz", h.CheckHealthz)
	return h
}

func (h *HealthzHandler) CheckHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(requestid.OutgoingContext(r.Context()), 3*time.Second)
	defer cancel()
	type result struct {
		name    string
		serving bool
	}
	results := make(chan result, 2)
	var microserviceList = []Microservice{
		{"catalog-svc", catalogpb.CatalogService_ServiceDesc.ServiceName, h.catalog},
		{"orders-svc", orderspb.OrdersService_ServiceDesc.ServiceName, h.orders},
	}

	for _, backend := range microserviceList {
		go func() {
			reply, err := backend.client.Check(ctx, &healthpb.HealthCheckRequest{Service: backend.service}, grpc.WaitForReady(true))
			if err != nil {
				slog.ErrorContext(ctx, "gRPC health check failed",
					"request_id", requestid.ID(ctx),
					"backend", backend.name,
					"service", backend.service,
					"error", err,
				)
			} else if reply.GetStatus() != healthpb.HealthCheckResponse_SERVING {
				slog.WarnContext(ctx, "gRPC backend is not serving",
					"request_id", requestid.ID(ctx),
					"backend", backend.name,
					"service", backend.service,
					"status", reply.GetStatus().String(),
				)
			}
			results <- result{backend.name, err == nil && reply.GetStatus() == healthpb.HealthCheckResponse_SERVING}
		}()
	}
	for range 2 {
		select {
		case result := <-results:
			if !result.serving {
				_ = response.Body(w, response.ErrorBody{Error: "unavailable", Message: result.name + " is not serving"}, http.StatusServiceUnavailable)
				return
			}
		case <-ctx.Done():
			_ = response.Body(w, response.ErrorBody{Error: "unavailable", Message: "gRPC health check did not complete"}, http.StatusServiceUnavailable)
			return
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte("ok")); err != nil {
		log.Printf("write health response: %v", err)
	}
}
