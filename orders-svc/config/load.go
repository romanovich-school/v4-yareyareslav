package config

import (
	"fmt"
	"os"
)

type GRPCConfig struct {
	Addr string
}

type DatabaseConfig struct {
	URL string
}

type CacheConfig struct {
	URL string
}

type KafkaConfig struct {
	Brokers string
}

type Config struct {
	GRPC     GRPCConfig
	Database DatabaseConfig
	Cache    CacheConfig
	Kafka    KafkaConfig
}

func Load() (*Config, error) {
	grpcAddr := getEnv("GRPC_ADDR", ":50052")

	databaseUrl, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		return nil, fmt.Errorf("Database is required")
	}

	cacheUrl, ok := os.LookupEnv("REDIS_URL")
	if !ok {
		return nil, fmt.Errorf("Cache is required")
	}

	cfg := &Config{
		GRPC: GRPCConfig{
			Addr: grpcAddr,
		},
		Database: DatabaseConfig{
			URL: databaseUrl,
		},
		Cache: CacheConfig{
			URL: cacheUrl,
		},
		Kafka: KafkaConfig{
			Brokers: "",
		},
	}

	return cfg, nil
}

func getEnv(env, fallback string) string {
	if value, ok := os.LookupEnv(env); ok {
		return value
	}
	return fallback
}
