package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"marketplace/gateway/config"
	"marketplace/gateway/internal/auth"
	"marketplace/gateway/internal/healthz"
	"marketplace/gateway/internal/listings"
	"marketplace/gateway/internal/orders"
	"marketplace/gateway/pkg/jwt"
	"marketplace/gen/catalogpb"
	"marketplace/gen/orderspb"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-playground/validator"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func connectToDB(databaseURL string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pool, err := connectToDB(cfg.Database.URL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if err := runMigrations(cfg.Database.URL); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	r := chi.NewRouter()
	validate := validator.New()

	jwtManager := jwt.NewManager(cfg.JWT.Secret, 15*time.Minute, 7*24*time.Hour)
	jwtMiddleware := jwt.NewJWTMiddleware(jwtManager)
	authConfig := config.NewAuthConfig(jwtMiddleware)

	healthz.NewHealthzHandler(r)

	authDBRepository := auth.NewAuthDBRepository(pool)
	authService := auth.NewAuthService(authDBRepository, jwtManager)
	auth.NewAuthHandler(r, validate, authService)

	catalogConn, err := grpc.NewClient(cfg.Catalog.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("create catalog client: %v", err)
	}
	defer catalogConn.Close()
	listingsService := listings.NewListingsService(catalogpb.NewCatalogServiceClient(catalogConn))
	listings.NewListingsHandler(r, validate, authConfig, listingsService)

	ordersConn, err := grpc.NewClient(cfg.Orders.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("create orders client: %v", err)
	}
	defer ordersConn.Close()
	ordersService := orders.NewOrdersService(orderspb.NewOrdersServiceClient(ordersConn))
	orders.NewOrdersHandler(r, validate, authConfig, ordersService)

	server := http.Server{
		Addr:    cfg.HTTP.Addr,
		Handler: r,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
