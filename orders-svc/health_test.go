package main

import (
	"context"
	"marketplace/gen/catalogpb"
	"marketplace/gen/orderspb"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func healthTestClient(t *testing.T, service healthpb.HealthServer) (healthpb.HealthClient, *grpc.Server) {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	healthpb.RegisterHealthServer(server, service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	conn, err := grpc.NewClient("passthrough:///health-test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return healthpb.NewHealthClient(conn), server
}

func TestOrdersHealthTracksCatalog(t *testing.T) {
	catalog := health.NewServer()
	catalogClient, catalogServer := healthTestClient(t, catalog)
	orders := newOrdersHealthServer(catalogClient)
	client, _ := healthTestClient(t, orders)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	check := func(want healthpb.HealthCheckResponse_ServingStatus) {
		t.Helper()
		for _, service := range []string{"", orderspb.OrdersService_ServiceDesc.ServiceName} {
			reply, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: service})
			if err != nil || reply.GetStatus() != want {
				t.Fatalf("service %q: response %v, error %v; want %s", service, reply, err, want)
			}
		}
	}

	check(healthpb.HealthCheckResponse_NOT_SERVING)
	for _, state := range []healthpb.HealthCheckResponse_ServingStatus{
		healthpb.HealthCheckResponse_SERVING,
		healthpb.HealthCheckResponse_NOT_SERVING,
		healthpb.HealthCheckResponse_UNKNOWN,
		healthpb.HealthCheckResponse_SERVING,
	} {
		catalog.SetServingStatus(catalogpb.CatalogService_ServiceDesc.ServiceName, state)
		want := healthpb.HealthCheckResponse_NOT_SERVING
		if state == healthpb.HealthCheckResponse_SERVING {
			want = state
		}
		check(want)
	}
	if _, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: "missing"}); status.Code(err) != codes.NotFound {
		t.Fatalf("unknown service: error %v; want NotFound", err)
	}

	catalogServer.Stop()
	check(healthpb.HealthCheckResponse_NOT_SERVING)
	orders.Shutdown()
	check(healthpb.HealthCheckResponse_NOT_SERVING)
}
