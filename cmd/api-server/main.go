package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/hackgods/distributed-appointment-scheduling/internal/config"
	"github.com/hackgods/distributed-appointment-scheduling/internal/db"
	"github.com/hackgods/distributed-appointment-scheduling/internal/appointment"
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
	_ = repo

	fmt.Printf("Config: appointment_ttl=%s lock_ttl=%s shutdown_timeout=%s\n",
		cfg.AppointmentTTL, cfg.LockTTL, cfg.ShutdownTimeout)

	<-rootCtx.Done()

	log.Println("shutting down api-server")
}
