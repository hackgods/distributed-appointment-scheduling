package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/hackgods/distributed-appointment-scheduling/internal/api"
	"github.com/hackgods/distributed-appointment-scheduling/internal/appointment"
	"github.com/hackgods/distributed-appointment-scheduling/internal/config"
	"github.com/hackgods/distributed-appointment-scheduling/internal/db"
	redisclient "github.com/hackgods/distributed-appointment-scheduling/internal/redis"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("api-server starting up")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	log.Printf("running in env=%s http_port=%s", cfg.Env, cfg.HTTPPort)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Connect Postgres
	pgCtx, cancelPg := context.WithTimeout(rootCtx, 10*time.Second)
	pgPool, err := db.ConnectPostgres(pgCtx, cfg.PostgresDSN)
	cancelPg()
	if err != nil {
		log.Fatalf("postgres connection error: %v", err)
	}
	defer pgPool.Close()
	log.Println("connected to Postgres")

	// Connect Redis
	rdb, err := redisclient.NewRedisClient(cfg.RedisAddr, cfg.RedisUsername, cfg.RedisPassword)
	if err != nil {
		log.Fatalf("redis connection error: %v", err)
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Printf("error closing redis: %v", err)
		}
	}()
	log.Println("connected to Redis")

	repo := appointment.NewPgRepository(pgPool)
	locker := redisclient.NewRedisSlotLocker(rdb, cfg.LockTTL)
	svc := appointment.NewService(repo, locker, cfg)

	router := api.NewRouter(svc)

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("HTTP server listening on :%s", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	fmt.Printf("Config: appointment_ttl=%s lock_ttl=%s shutdown_timeout=%s\n",
		cfg.AppointmentTTL, cfg.LockTTL, cfg.ShutdownTimeout)

	<-rootCtx.Done()
	log.Println("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	} else {
		log.Println("http server shut down gracefully")
	}

	log.Println("shutting down api-server")
}
