package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env             string        // dev, prod
	HTTPPort        string        // default 8080
	PostgresDSN     string        // required
	RedisAddr       string        // host:port
	RedisUsername   string        // redis username
	RedisPassword   string        // redis password
	AppointmentTTL  time.Duration // how long a pending appointment stays reserved
	LockTTL         time.Duration // how long a Redis slot lock lives
	ShutdownTimeout time.Duration // graceful shutdown timeout
	WorkerInterval  time.Duration // how often the expiry worker runs
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		Env:             getEnv("APP_ENV", "dev"),
		HTTPPort:        getEnv("HTTP_PORT", "8080"),
		PostgresDSN:     os.Getenv("POSTGRES_DSN"),
		AppointmentTTL:  getDuration("APPOINTMENT_TTL", 10*time.Minute),
		LockTTL:         getDuration("LOCK_TTL", 5*time.Second),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		WorkerInterval:  getDuration("WORKER_INTERVAL", time.Minute),
	}

	if cfg.PostgresDSN == "" {
		return Config{}, errors.New("POSTGRES_DSN is required")
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		addr, username, password, err := parseRedisURL(redisURL)
		if err != nil {
			return Config{}, fmt.Errorf("invalid REDIS_URL: %w", err)
		}
		cfg.RedisAddr = addr
		cfg.RedisUsername = username
		cfg.RedisPassword = password
	} else {
		cfg.RedisAddr = getEnv("REDIS_ADDR", "127.0.0.1:6379")
		cfg.RedisUsername = getEnv("REDIS_USERNAME", "")
		cfg.RedisPassword = getEnv("REDIS_PASSWORD", "")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		fmt.Fprintf(os.Stderr, "invalid duration for %s=%q, using default %s\n", key, v, def)
	}
	return def
}

// parseRedisURL parses redis://user:password@host:port
func parseRedisURL(raw string) (addr, username, password string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", err
	}

	addr = u.Host

	if u.User != nil {
		username = u.User.Username()
		pw, _ := u.User.Password()
		password = pw
	}

	return addr, username, password, nil
}
