package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr          string
	HTTPReadTimeout   time.Duration
	HTTPHeaderTimeout time.Duration
	HTTPWriteTimeout  time.Duration
	HTTPIdleTimeout   time.Duration
	LogLevel          string
	ShutdownTimeout   time.Duration

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
	cfg := &Config{}

	var err error

	cfg.HTTPAddr, err = requiredString("HTTP_ADDR")
	if err != nil {
		return nil, err
	}

	cfg.LogLevel, err = requiredString("LOG_LEVEL")
	if err != nil {
		return nil, err
	}

	cfg.HTTPReadTimeout, err = requiredDuration("HTTP_READ_TIMEOUT")
	if err != nil {
		return nil, err
	}

	cfg.HTTPHeaderTimeout, err = requiredDuration("HTTP_READ_HEADER_TIMEOUT")
	if err != nil {
		return nil, err
	}

	cfg.HTTPWriteTimeout, err = requiredDuration("HTTP_WRITE_TIMEOUT")
	if err != nil {
		return nil, err
	}

	cfg.HTTPIdleTimeout, err = requiredDuration("HTTP_IDLE_TIMEOUT")
	if err != nil {
		return nil, err
	}

	cfg.ShutdownTimeout, err = requiredDuration("SHUTDOWN_TIMEOUT")
	if err != nil {
		return nil, err
	}

	cfg.DB.URL, err = requiredString("DATABASE_URL")
	if err != nil {
		return nil, err
	}

	cfg.DB.MaxConns, err = requiredInt32("DATABASE_MAX_CONNS")
	if err != nil {
		return nil, err
	}

	cfg.DB.MinConns, err = requiredInt32("DATABASE_MIN_CONNS")
	if err != nil {
		return nil, err
	}

	if cfg.DB.MaxConns < 1 {
		return nil, fmt.Errorf("DATABASE_MAX_CONNS must be at least 1")
	}
	if cfg.DB.MinConns < 0 || cfg.DB.MinConns > cfg.DB.MaxConns {
		return nil, fmt.Errorf("DATABASE_MIN_CONNS must be between 0 and DATABASE_MAX_CONNS")
	}

	cfg.DB.MaxConnLifetime, err = requiredDuration("DATABASE_MAX_CONN_LIFETIME")
	if err != nil {
		return nil, err
	}

	cfg.DB.ConnectTimeout, err = requiredDuration("DATABASE_CONNECT_TIMEOUT")
	if err != nil {
		return nil, err
	}

	cfg.DB.QueryTimeout, err = requiredDuration("DATABASE_QUERY_TIMEOUT")
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func requiredString(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}

	return value, nil
}

func requiredDuration(key string) (time.Duration, error) {
	value, err := requiredString(key)
	if err != nil {
		return 0, err
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration for %s: %w", key, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}

	return duration, nil
}

func requiredInt32(key string) (int32, error) {
	value, err := requiredString(key)
	if err != nil {
		return 0, err
	}

	number, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid integer for %s: %w", key, err)
	}

	return int32(number), nil
}
