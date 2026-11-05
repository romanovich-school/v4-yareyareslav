package main

import (
	"context"
	"log/slog"
	"marketplace/gen/catalogpb"
	"marketplace/gen/orderspb"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type ordersHealthServer struct {
	healthpb.UnimplementedHealthServer
	local   *health.Server
	catalog healthpb.HealthClient
}

func newOrdersHealthServer(catalog healthpb.HealthClient) *ordersHealthServer {
	local := health.NewServer()
	local.SetServingStatus(orderspb.OrdersService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	return &ordersHealthServer{local: local, catalog: catalog}
}

func (s *ordersHealthServer) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	local, err := s.local.Check(ctx, req)
	if err != nil || local.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return local, err
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	reply, err := s.catalog.Check(checkCtx, &healthpb.HealthCheckRequest{
		Service: catalogpb.CatalogService_ServiceDesc.ServiceName,
	}, grpc.WaitForReady(true))
	if err != nil {
		slog.WarnContext(ctx, "catalog health check failed", "error", err)
	}
	if err != nil || reply.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_NOT_SERVING}, nil
	}
	return s.local.Check(ctx, req)
}

func (s *ordersHealthServer) Shutdown() {
	s.local.Shutdown()
}
