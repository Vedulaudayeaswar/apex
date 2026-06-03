package config

import (
	"os"
	"strconv"
	"time"
)

func Get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func GetInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func GetDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

type ServiceConfig struct {
	StoreID          string
	KafkaBrokers     string
	RedisURL         string
	PostgresDSN      string
	HTTPPort         string
	MetricsPort      string
	OTelEndpoint     string
	ServiceName      string
}

func LoadService(name string) ServiceConfig {
	return ServiceConfig{
		StoreID:      Get("STORE_ID", "STORE_BLR_002"),
		KafkaBrokers: Get("KAFKA_BROKERS", "kafka:9092"),
		RedisURL:     Get("REDIS_URL", "redis://redis:6379/0"),
		PostgresDSN:  Get("POSTGRES_DSN", "postgres://retail:retail@postgres:5432/retail_intel?sslmode=disable"),
		HTTPPort:     Get("HTTP_PORT", "8080"),
		MetricsPort:  Get("METRICS_PORT", "9090"),
		OTelEndpoint: Get("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		ServiceName:  name,
	}
}
