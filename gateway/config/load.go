package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type HTTPConfig struct {
	Addr string
}

type DatabaseConfig struct {
	URL string
}

type JWTConfig struct {
	Secret string
}

type CatalogConfig struct {
	Addr string
}

type Config struct {
	HTTP     HTTPConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Catalog  CatalogConfig
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	catalogAddr := os.Getenv("CATALOG_GRPC_ADDR")
	if catalogAddr == "" {
		return nil, fmt.Errorf("CATALOG_GRPC_ADDR is required")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		HTTP:     HTTPConfig{Addr: ":" + port},
		Database: DatabaseConfig{URL: databaseURL},
		JWT:      JWTConfig{Secret: jwtSecret},
		Catalog:  CatalogConfig{Addr: catalogAddr},
	}, nil
}
