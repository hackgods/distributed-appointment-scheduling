package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hackgods/distributed-appointment-scheduling/internal/config"
	"github.com/hackgods/distributed-appointment-scheduling/internal/db"
)

type SimConfig struct {
	APIBaseURL   string
	Duration     time.Duration
	Workers      int
	BookingRatio float64
	ConfirmRatio float64
	ReadRatio    float64
	PatientLimit int
	SlotLimit    int
	PostgresDSN  string
}

type DataPool struct {
	Patients     []uuid.UUID
	Slots        []uuid.UUID
	mu           sync.RWMutex
	appointments []uuid.UUID // Thread-safe list of created appointment IDs
}

func (dp *DataPool) AddAppointment(id uuid.UUID) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.appointments = append(dp.appointments, id)
}

func (dp *DataPool) GetRandomAppointment() (uuid.UUID, bool) {
	dp.mu.RLock()
	defer dp.mu.RUnlock()
	if len(dp.appointments) == 0 {
		return uuid.Nil, false
	}
	idx := rand.Intn(len(dp.appointments))
	return dp.appointments[idx], true
}

type OperationMetrics struct {
	Total     int64
	Success   int64
	Conflict  int64
	Error     int64
	Latencies []time.Duration
	mu        sync.Mutex
}

func (om *OperationMetrics) Record(latency time.Duration, success bool, conflict bool) {
	atomic.AddInt64(&om.Total, 1)
	if success {
		atomic.AddInt64(&om.Success, 1)
	} else if conflict {
		atomic.AddInt64(&om.Conflict, 1)
	} else {
		atomic.AddInt64(&om.Error, 1)
	}

	om.mu.Lock()
	om.Latencies = append(om.Latencies, latency)
	om.mu.Unlock()
}

func (om *OperationMetrics) Stats() (avg, min, max, p50, p95 time.Duration) {
	om.mu.Lock()
	defer om.mu.Unlock()

	if len(om.Latencies) == 0 {
		return 0, 0, 0, 0, 0
	}

	latencies := make([]time.Duration, len(om.Latencies))
	copy(latencies, om.Latencies)

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}

	avg = sum / time.Duration(len(latencies))
	min = latencies[0]
	max = latencies[len(latencies)-1]

	if len(latencies) > 0 {
		p50Idx := len(latencies) * 50 / 100
		if p50Idx >= len(latencies) {
			p50Idx = len(latencies) - 1
		}
		p50 = latencies[p50Idx]

		p95Idx := len(latencies) * 95 / 100
		if p95Idx >= len(latencies) {
			p95Idx = len(latencies) - 1
		}
		p95 = latencies[p95Idx]
	}

	return avg, min, max, p50, p95
}

type Metrics struct {
	Booking       OperationMetrics
	Confirm       OperationMetrics
	ReadByID      OperationMetrics
	ListByPatient OperationMetrics
	ListBySlot    OperationMetrics
}

type Simulator struct {
	config  SimConfig
	pool    *DataPool
	client  *http.Client
	metrics Metrics
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("simulator starting")

	cfg := loadConfig()
	if err := validateConfig(cfg); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	log.Printf("config: duration=%s workers=%d booking=%.2f confirm=%.2f read=%.2f",
		cfg.Duration, cfg.Workers, cfg.BookingRatio, cfg.ConfirmRatio, cfg.ReadRatio)

	// Load data from Postgres
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pgPool, err := db.ConnectPostgres(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer pgPool.Close()

	dataPool, err := loadDataPool(ctx, pgPool, cfg)
	if err != nil {
		log.Fatalf("load data pool: %v", err)
	}

	log.Printf("loaded: %d patients, %d slots", len(dataPool.Patients), len(dataPool.Slots))

	sim := &Simulator{
		config: cfg,
		pool:   dataPool,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// Run simulation
	sim.Run()

	// Print report
	sim.PrintReport()
}

func loadConfig() SimConfig {
	baseCfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load base config: %v", err)
	}

	cfg := SimConfig{
		APIBaseURL:   getEnv("SIM_API_BASE_URL", "http://localhost:8080"),
		Duration:     getDuration("SIM_DURATION", 30*time.Second),
		Workers:      getInt("SIM_WORKERS", 10),
		BookingRatio: getFloat("SIM_BOOKING_RATIO", 0.5),
		ConfirmRatio: getFloat("SIM_CONFIRM_RATIO", 0.2),
		ReadRatio:    getFloat("SIM_READ_RATIO", 0.3),
		PatientLimit: getInt("SIM_PATIENT_LIMIT", 4000),
		SlotLimit:    getInt("SIM_SLOT_LIMIT", 2400),
		PostgresDSN:  baseCfg.PostgresDSN,
	}

	// Normalize ratios
	total := cfg.BookingRatio + cfg.ConfirmRatio + cfg.ReadRatio
	if total > 0 {
		cfg.BookingRatio /= total
		cfg.ConfirmRatio /= total
		cfg.ReadRatio /= total
	}

	return cfg
}

func validateConfig(cfg SimConfig) error {
	if cfg.PostgresDSN == "" {
		return fmt.Errorf("POSTGRES_DSN is required (set in .env or environment)")
	}
	if cfg.Workers <= 0 {
		return fmt.Errorf("SIM_WORKERS must be > 0")
	}
	if cfg.Duration <= 0 {
		return fmt.Errorf("SIM_DURATION must be > 0")
	}
	return nil
}

func loadDataPool(ctx context.Context, pool *pgxpool.Pool, cfg SimConfig) (*DataPool, error) {
	dataPool := &DataPool{}

	// Load patients
	rows, err := pool.Query(ctx, `
		SELECT id FROM patients LIMIT $1
	`, cfg.PatientLimit)
	if err != nil {
		return nil, fmt.Errorf("load patients: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		dataPool.Patients = append(dataPool.Patients, id)
	}

	// Load open slots
	rows, err = pool.Query(ctx, `
		SELECT id FROM appointment_slots 
		WHERE status = 'open' AND start_time > now() 
		LIMIT $1
	`, cfg.SlotLimit)
	if err != nil {
		return nil, fmt.Errorf("load slots: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		dataPool.Slots = append(dataPool.Slots, id)
	}

	if len(dataPool.Patients) == 0 {
		return nil, fmt.Errorf("no patients loaded")
	}
	if len(dataPool.Slots) == 0 {
		return nil, fmt.Errorf("no slots loaded")
	}

	return dataPool, nil
}

func (s *Simulator) Run() {
	ctx, cancel := context.WithTimeout(context.Background(), s.config.Duration)
	defer cancel()

	log.Printf("starting simulation for %s with %d workers", s.config.Duration, s.config.Workers)

	var wg sync.WaitGroup
	for i := 0; i < s.config.Workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			s.worker(ctx, workerID)
		}(i)
	}

	wg.Wait()
	log.Println("simulation complete")
}

func (s *Simulator) worker(ctx context.Context, workerID int) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Select operation based on ratios
			r := rng.Float64()
			if r < s.config.BookingRatio {
				s.doBooking(ctx, rng)
			} else if r < s.config.BookingRatio+s.config.ConfirmRatio {
				s.doConfirm(ctx, rng)
			} else {
				// Read operations - distribute evenly
				readOp := rng.Intn(3)
				switch readOp {
				case 0:
					s.doReadByID(ctx, rng)
				case 1:
					s.doListByPatient(ctx, rng)
				case 2:
					s.doListBySlot(ctx, rng)
				}
			}
		}
	}
}

func (s *Simulator) doBooking(ctx context.Context, rng *rand.Rand) {
	if len(s.pool.Slots) == 0 || len(s.pool.Patients) == 0 {
		return
	}

	slotID := s.pool.Slots[rng.Intn(len(s.pool.Slots))]
	patientID := s.pool.Patients[rng.Intn(len(s.pool.Patients))]

	start := time.Now()

	reqBody := map[string]string{
		"slot_id":    slotID.String(),
		"patient_id": patientID.String(),
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequestWithContext(ctx, "POST", s.config.APIBaseURL+"/appointments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	latency := time.Since(start)

	success := false
	conflict := false

	if err == nil {
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusCreated {
			success = true
			// Parse response to get appointment ID
			var apptResp struct {
				ID uuid.UUID `json:"id"`
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			if len(bodyBytes) > 0 {
				json.Unmarshal(bodyBytes, &apptResp)
				if apptResp.ID != uuid.Nil {
					s.pool.AddAppointment(apptResp.ID)
				}
			}
		} else if resp.StatusCode == http.StatusConflict {
			conflict = true
		}
	}

	s.metrics.Booking.Record(latency, success, conflict)
}

func (s *Simulator) doConfirm(ctx context.Context, rng *rand.Rand) {
	apptID, ok := s.pool.GetRandomAppointment()
	if !ok {
		return
	}

	start := time.Now()

	req, _ := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/appointments/%s/confirm", s.config.APIBaseURL, apptID.String()), nil)

	resp, err := s.client.Do(req)
	latency := time.Since(start)

	success := false
	conflict := false

	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			success = true
		} else if resp.StatusCode == http.StatusConflict {
			conflict = true
		}
	}

	s.metrics.Confirm.Record(latency, success, conflict)
}

func (s *Simulator) doReadByID(ctx context.Context, rng *rand.Rand) {
	apptID, ok := s.pool.GetRandomAppointment()
	if !ok {
		return
	}

	start := time.Now()

	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/appointments/%s", s.config.APIBaseURL, apptID.String()), nil)

	resp, err := s.client.Do(req)
	latency := time.Since(start)

	success := false
	if err == nil {
		defer resp.Body.Close()
		success = resp.StatusCode == http.StatusOK
	}

	s.metrics.ReadByID.Record(latency, success, false)
}

func (s *Simulator) doListByPatient(ctx context.Context, rng *rand.Rand) {
	if len(s.pool.Patients) == 0 {
		return
	}

	patientID := s.pool.Patients[rng.Intn(len(s.pool.Patients))]

	start := time.Now()

	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/appointments?patient_id=%s&limit=20&offset=0", s.config.APIBaseURL, patientID.String()), nil)

	resp, err := s.client.Do(req)
	latency := time.Since(start)

	success := false
	if err == nil {
		defer resp.Body.Close()
		success = resp.StatusCode == http.StatusOK
	}

	s.metrics.ListByPatient.Record(latency, success, false)
}

func (s *Simulator) doListBySlot(ctx context.Context, rng *rand.Rand) {
	if len(s.pool.Slots) == 0 {
		return
	}

	slotID := s.pool.Slots[rng.Intn(len(s.pool.Slots))]

	start := time.Now()

	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/appointments?slot_id=%s", s.config.APIBaseURL, slotID.String()), nil)

	resp, err := s.client.Do(req)
	latency := time.Since(start)

	success := false
	if err == nil {
		defer resp.Body.Close()
		success = resp.StatusCode == http.StatusOK
	}

	s.metrics.ListBySlot.Record(latency, success, false)
}

func (s *Simulator) PrintReport() {
	fmt.Println("\n" + repeat("=", 80))
	fmt.Println("SIMULATION REPORT")
	fmt.Println(repeat("=", 80))
	fmt.Printf("Duration: %s\n", s.config.Duration)
	fmt.Printf("Workers: %d\n", s.config.Workers)
	fmt.Println()

	printOperationReport("Booking", &s.metrics.Booking)
	printOperationReport("Confirm", &s.metrics.Confirm)
	printOperationReport("Read by ID", &s.metrics.ReadByID)
	printOperationReport("List by Patient", &s.metrics.ListByPatient)
	printOperationReport("List by Slot", &s.metrics.ListBySlot)
}

func printOperationReport(name string, om *OperationMetrics) {
	total := atomic.LoadInt64(&om.Total)
	if total == 0 {
		return
	}

	success := atomic.LoadInt64(&om.Success)
	conflict := atomic.LoadInt64(&om.Conflict)
	error := atomic.LoadInt64(&om.Error)

	avg, min, max, p50, p95 := om.Stats()

	fmt.Printf("%s:\n", name)
	fmt.Printf("  Total: %d\n", total)
	fmt.Printf("  Success: %d (%.1f%%)\n", success, float64(success)/float64(total)*100)
	if conflict > 0 {
		fmt.Printf("  Conflicts: %d (%.1f%%)\n", conflict, float64(conflict)/float64(total)*100)
	}
	if error > 0 {
		fmt.Printf("  Errors: %d (%.1f%%)\n", error, float64(error)/float64(total)*100)
	}
	fmt.Printf("  Latency: avg=%s min=%s max=%s p50=%s p95=%s\n",
		avg.Round(time.Millisecond), min.Round(time.Millisecond), max.Round(time.Millisecond),
		p50.Round(time.Millisecond), p95.Round(time.Millisecond))
	fmt.Println()
}

// Helper functions

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func repeat(s string, n int) string {
	return strings.Repeat(s, n)
}
