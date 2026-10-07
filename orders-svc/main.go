package main

import (
	"context"
	"embed"
	"log"
	"marketplace/gen/orderspb"
	"marketplace/orders-svc/config"
	"net"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
)

type ordersServer struct {
	orderspb.UnimplementedOrdersServiceServer
}

//go:embed migrations/*.sql
var migrationFiles embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// DATABASE
	pool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("unable to create pool: %v", err)
	}
	defer pool.Close()

	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		log.Fatalf("unable to open embedded migrations: %v", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, cfg.Database.URL)
	if err != nil {
		source.Close()
		log.Fatalf("migrations failed to initialize: %v", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("failed to apply migrations: %v", err)
	}

	// GRPC
	listener, err := net.Listen("tcp", cfg.GRPC.Addr)
	if err != nil {
		log.Fatalf("unable to listen grpc: %v", err)
	}
	defer listener.Close()

	server := grpc.NewServer()

	orderspb.RegisterOrdersServiceServer(server, &ordersServer{})
	log.Printf("gRPC server listening on %s", cfg.GRPC.Addr)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}

}
