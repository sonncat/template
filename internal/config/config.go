package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPAddr        string
	LogLevel        string
	ShutdownTimeout time.Duration

	DB Database
}

type Database struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}

	cfg.DB = Database{
		URL:             os.Getenv("DATABASE_URL"),
		MaxConns:        int32(getInt("DATABASE_MAX_CONNS", 10)),
		MinConns:        int32(getInt("DATABASE_MIN_CONNS", 2)),
		MaxConnLifetime: getDuration("DATABASE_MAX_CONN_LIFETIME", 30*time.Minute),
		ConnectTimeout:  getDuration("DATABASE_CONNECT_TIMEOUT", 5*time.Second),
		QueryTimeout:    getDuration("DATABASE_QUERY_TIMEOUT", 3*time.Second),
	}

	if cfg.DB.URL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return fallback
}
