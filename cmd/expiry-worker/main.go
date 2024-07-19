package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/hackgods/distributed-appointment-scheduling/internal/appointment"
	"github.com/hackgods/distributed-appointment-scheduling/internal/config"
	"github.com/hackgods/distributed-appointment-scheduling/internal/db"
	redisclient "github.com/hackgods/distributed-appointment-scheduling/internal/redis"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("expiry-worker starting up")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	log.Printf("running expiry worker in env=%s interval=%s", cfg.Env, cfg.WorkerInterval)

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

	// Run once at startup
	runOnce(rootCtx, svc)

	ticker := time.NewTicker(cfg.WorkerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rootCtx.Done():
			log.Println("shutdown signal received, stopping expiry worker")
			return
		case <-ticker.C:
			runOnce(rootCtx, svc)
		}
	}
}

func runOnce(ctx context.Context, svc *appointment.Service) {
	runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	start := time.Now()
	if err := svc.ExpirePendingAppointments(runCtx); err != nil {
		log.Printf("expiry run error: %v", err)
		return
	}
	log.Printf("expiry run complete in %s", time.Since(start))
}
