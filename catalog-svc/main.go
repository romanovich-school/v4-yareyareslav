package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"log/slog"
	"marketplace/catalog-svc/config"
	"marketplace/catalog-svc/internal/listings/repository"
	"marketplace/catalog-svc/internal/listings/service"
	"marketplace/catalog-svc/internal/listings/service/adapter"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	parsedRedisUrl, err := redis.ParseURL(cfg.Cache.URL)
	if err != nil {
		return fmt.Errorf("redis url parse: %w", err)
	}
	parsedRedisUrl.MaxRetries = -1
	parsedRedisUrl.DialerRetries = 1
	parsedRedisUrl.ContextTimeoutEnabled = true
	parsedRedisUrl.DialTimeout = 200 * time.Millisecond
	parsedRedisUrl.ReadTimeout = 200 * time.Millisecond
	parsedRedisUrl.WriteTimeout = 200 * time.Millisecond
	parsedRedisUrl.PoolTimeout = 200 * time.Millisecond

	rdb := redis.NewClient(parsedRedisUrl)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.WarnContext(ctx, "Error starting redis")
	}

	adaptedRdb := adapter.NewRedisAdapter(rdb)

	// SERVICE
	listingsPGRepository := repository.NewPGRepository(pool)
	_ = service.NewCatalogService(listingsPGRepository, adaptedRdb)

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
