package api

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	pgPool    *pgxpool.Pool
	redis     *redis.Client
	env       string
	version   string
}

func NewHealthHandler(pgPool *pgxpool.Pool, redis *redis.Client, env, version string) *HealthHandler {
	return &HealthHandler{
		pgPool:  pgPool,
		redis:   redis,
		env:     env,
		version: version,
	}
}

type LivenessResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
	Env     string `json:"env,omitempty"`
}

type ReadinessResponse struct {
	Status      string                 `json:"status"`
	Version     string                 `json:"version,omitempty"`
	Env         string                 `json:"env,omitempty"`
	Dependencies map[string]string     `json:"dependencies"`
}

func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	resp := LivenessResponse{
		Status:  "ok",
		Version: h.version,
		Env:     h.env,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	deps := make(map[string]string)
	status := "ok"

	// Check Postgres
	pgCtx, pgCancel := context.WithTimeout(ctx, 1*time.Second)
	err := h.pgPool.Ping(pgCtx)
	pgCancel()
	if err != nil {
		deps["postgres"] = "down"
		status = "error"
	} else {
		deps["postgres"] = "ok"
	}

	// Check Redis
	redisCtx, redisCancel := context.WithTimeout(ctx, 1*time.Second)
	err = h.redis.Ping(redisCtx).Err()
	redisCancel()
	if err != nil {
		deps["redis"] = "down"
		if status == "ok" {
			status = "degraded"
		} else {
			status = "error"
		}
	} else {
		deps["redis"] = "ok"
	}

	resp := ReadinessResponse{
		Status:      status,
		Version:     h.version,
		Env:         h.env,
		Dependencies: deps,
	}

	httpStatus := http.StatusOK
	if status == "error" {
		httpStatus = http.StatusServiceUnavailable
	}

	writeJSON(w, httpStatus, resp)
}

