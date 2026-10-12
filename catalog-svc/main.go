package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"marketplace/catalog-svc/config"
	"marketplace/catalog-svc/internal/listings/repository"
	"marketplace/catalog-svc/internal/listings/service"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func run(ctx context.Context, errChan chan error) error {
	workerStopped := true
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// DATABASE
	pool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("unable to create pool: %v", err)
	}
	defer func() {
		if workerStopped {
			pool.Close()
		}
	}()

	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("unable to open embedded migrations: %v", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, cfg.Database.URL)
	if err != nil {
		source.Close()
		return fmt.Errorf("migrations failed to initialize: %v", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %v", err)
	}

	// REDIS
	_ = redis.NewClient(&redis.Options{
		Addr:     "localhost:" + cfg.Cache.URL,
		Password: cfg.Cache.Password,
		DB:       0,
	})

	// SERVICE
	listingsPGRepository := repository.NewPGRepository(pool)
	_ = service.NewCatalogService(listingsPGRepository)

	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	errChan := make(chan error, 1)
	if err := run(ctx, errChan); err != nil {
		log.Printf("ERROR: catalog service failed: %v", err)
		stop()
		os.Exit(1)
	}

	fmt.Println("Application shutdown complete.")
}
