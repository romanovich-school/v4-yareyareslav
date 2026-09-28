package main

import (
	"context"
	"fmt"
	"log"
	"marketplace/gateway/config"
	"marketplace/gateway/internal/auth"
	"marketplace/gateway/internal/healthz"
	"marketplace/gateway/internal/listings"
	"marketplace/gateway/internal/orders"
	"marketplace/gateway/pkg/jwt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-playground/validator"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type EnvConf struct {
	databaseURL string
	jwtSecret string
	port string
}

func checkEnv() (EnvConf) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		return EnvConf{}
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
		return EnvConf{}
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWTSecret is dull")
		return EnvConf{}
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return EnvConf{
		databaseURL: databaseURL,
		jwtSecret: jwtSecret,
		port: ":" + port,
	}
}

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
	envs := checkEnv()

	pool, err := connectToDB(envs.databaseURL)
	if err != nil {
		log.Fatal(err);
	}

	if err := runMigrations(envs.databaseURL); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	r := chi.NewRouter()
	validate := validator.New()

	jwtManager := jwt.NewManager(envs.jwtSecret, 15 * time.Minute, 7 * 24 * time.Hour)
	jwtMiddleware := jwt.NewJWTMiddleware(jwtManager)
	authConfig := config.NewAuthConfig(jwtMiddleware)

	healthz.NewHealthzHandler(r)

	authDBRepository := auth.NewAuthDBRepository(pool)
	authService := auth.NewAuthService(authDBRepository, jwtManager)
	auth.NewAuthHandler(r, validate, authService)

	listingsService := listings.NewListingsService()
	listings.NewListingsHandler(r, validate, authConfig, listingsService)

	ordersService := orders.NewOrdersService()
	orders.NewOrdersHandler(r, validate, authConfig, ordersService)

	server := http.Server{
		Addr: envs.port,
		Handler: r,
	}
	server.ListenAndServe()
}