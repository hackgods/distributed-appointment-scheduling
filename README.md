# Distributed Hospital Appointment Scheduling System

A production-grade, distributed appointment scheduling system built in Go. This system handles concurrent appointment bookings with strong consistency guarantees, automatic expiry of pending appointments, and comprehensive event logging for audit trails.

## Overview

This is a microservice-based appointment scheduling system designed to handle high concurrency scenarios where multiple users might attempt to book the same time slot simultaneously. The system ensures data integrity through a combination of PostgreSQL database constraints, Redis distributed locking, and careful transaction management.

**Key Problem Solved**: Preventing double-booking when multiple users try to reserve the same appointment slot at the same time, while maintaining high availability and performance.

## Architecture

The system follows a **hexagonal (ports and adapters) architecture** with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────┐
│                    HTTP API Layer                        │
│  (Handlers, Router, Middleware, Request/Response)       │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│                 Service Layer                            │
│  (Business Logic, Domain Rules, Event Logging)          │
└──────────────────────┬──────────────────────────────────┘
                       │
        ┌──────────────┴──────────────┐
        │                              │
┌───────▼────────┐          ┌─────────▼─────────┐
│  Repository    │          │  Redis Locker     │
│  (PostgreSQL)  │          │  (Distributed)    │
└────────────────┘          └───────────────────┘
```

### Core Components

1. **API Server** (`cmd/api-server`) - HTTP REST API that handles appointment operations
2. **Expiry Worker** (`cmd/expiry-worker`) - Background service that automatically expires pending appointments
3. **Simulator** (`cmd/simulate`) - Load testing tool for validating system behavior under contention
4. **Seed Tool** (`cmd/seed`) - Database seeding utility for development and testing

### Design Principles

- **Strong Consistency**: Database-level constraints ensure only one confirmed appointment per slot
- **Distributed Locking**: Redis-based locks prevent race conditions in multi-instance deployments
- **Event Sourcing**: All state changes are logged for audit and debugging
- **Graceful Degradation**: System continues operating even if some components are temporarily unavailable
- **Observability**: Request ID tracking, structured logging, and health endpoints

## Features

### Appointment Management

- **Create Pending Appointments**: Reserve a slot for a patient with automatic expiry
- **Confirm Appointments**: Convert pending appointments to confirmed status
- **Automatic Expiry**: Background worker expires pending appointments after TTL
- **Conflict Prevention**: Distributed locking prevents double-booking

### Data Integrity

- **Database Constraints**: Partial unique indexes ensure only one confirmed appointment per slot
- **Transaction Safety**: All critical operations use database transactions
- **Status Validation**: Enforces valid state transitions (pending → confirmed → expired)

### Observability

- **Health Endpoints**: Liveness and readiness checks for orchestration
- **Structured Logging**: Request ID tracking across all operations
- **Event Logging**: Complete audit trail of all appointment state changes
- **Metrics**: Built-in simulation tool provides performance metrics

### Scalability

- **Stateless API**: API server can be horizontally scaled
- **Distributed Locks**: Redis-based locking works across multiple instances
- **Connection Pooling**: Efficient database and Redis connection management
- **Indexed Queries**: Optimized database indexes for read operations

## Use Cases

This system is ideal for:

- **Healthcare Scheduling**: Doctor appointments, clinic bookings
- **Service Booking**: Hair salons, consulting services, tutoring
- **Resource Reservation**: Meeting rooms, equipment rental
- **Event Registration**: Limited-capacity events, workshops

**When to use this system:**

- You need to prevent double-booking of time slots
- You require audit trails for compliance
- You need to handle high concurrent load
- You want automatic cleanup of abandoned reservations
- You need a production-ready, battle-tested solution

## Prerequisites

- **Go 1.23+** - The system is built with Go
- **PostgreSQL 12+** - Primary data store
- **Redis 6+** - Distributed locking and coordination
- **Make** (optional) - For convenience commands

## Installation

### Clone the Repository

```bash
git clone https://github.com/hackgods/distributed-appointment-scheduling.git
cd distributed-appointment-scheduling
```

### Install Dependencies

```bash
go mod download
```

### Database Setup

1. Create a PostgreSQL database:

```sql
CREATE DATABASE appointment_scheduling;
```

2. Run migrations (manually or via your preferred migration tool):

```bash
# Connect to your database and run migrations in order:
# internal/db/migrations/0001_init_core.sql
# internal/db/migrations/0002_slots_appointments.sql
# internal/db/migrations/0003_event_logs.sql
# internal/db/migrations/0004_read_indexes.sql
```

### Configuration

Create a `.env` file in the project root:

```env
# Database
POSTGRES_DSN=postgres://user:password@localhost:5432/appointment_scheduling

# Redis
REDIS_URL=redis://localhost:6379
# OR use separate variables:
# REDIS_ADDR=localhost:6379
# REDIS_USERNAME=
# REDIS_PASSWORD=

# Application
APP_ENV=dev
HTTP_PORT=8080

# Timeouts and TTLs
APPOINTMENT_TTL=10m
LOCK_TTL=5s
SHUTDOWN_TIMEOUT=10s
WORKER_INTERVAL=1m
```

The system automatically loads `.env` files using the `godotenv` package. Environment variables take precedence over `.env` file values.

## Building

### Build All Binaries

```bash
go build ./cmd/api-server
go build ./cmd/expiry-worker
go build ./cmd/simulate
go build ./cmd/seed
```

### Build Everything

```bash
go build ./...
```

## Running

### 1. Start the API Server

```bash
go run ./cmd/api-server
# or
./api-server
```

The API server will:

- Connect to PostgreSQL and Redis
- Start HTTP server on port 8080 (or configured port)
- Handle graceful shutdown on SIGINT/SIGTERM

### 2. Start the Expiry Worker

In a separate terminal:

```bash
go run ./cmd/expiry-worker
# or
./expiry-worker
```

The expiry worker:

- Runs periodically (default: every 1 minute)
- Finds and expires pending appointments past their TTL
- Logs expiry events for audit

### 3. Seed Test Data (Optional)

```bash
go run ./cmd/seed
# or
./seed
```

This creates:

- 100 clinicians
- 4000 patients
- (You'll need to create slots separately or via your application logic)

## API Documentation

### Base URL

```
http://localhost:8080
```

### Endpoints

#### Health Checks

**GET `/health/live`**

- Liveness probe for container orchestration
- Returns 200 if the process is running
- Does not check dependencies

**GET `/health/ready`**

- Readiness probe for container orchestration
- Checks PostgreSQL and Redis connectivity
- Returns 200 if ready, 503 if dependencies are down

#### Appointment Operations

**POST `/appointments`**
Create a new pending appointment.

Request:

```json
{
  "slot_id": "550e8400-e29b-41d4-a716-446655440000",
  "patient_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
}
```

Response (201 Created):

```json
{
  "id": "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
  "slot_id": "550e8400-e29b-41d4-a716-446655440000",
  "patient_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "status": "pending",
  "expires_at": "2024-01-15T10:20:00Z"
}
```

Error Responses:

- `400` - Invalid request body or UUID format
- `404` - Patient or slot not found
- `409` - Slot already booked or currently being booked
- `500` - Internal server error

**POST `/appointments/{id}/confirm`**
Confirm a pending appointment.

Response (200 OK):

```json
{
  "id": "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
  "slot_id": "550e8400-e29b-41d4-a716-446655440000",
  "patient_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "status": "confirmed",
  "expires_at": null
}
```

Error Responses:

- `400` - Invalid appointment ID
- `404` - Appointment not found
- `409` - Appointment expired or invalid status transition
- `500` - Internal server error

**GET `/appointments/{id}`**
Get a fully hydrated appointment with related entities.

Response (200 OK):

```json
{
  "id": "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
  "status": "confirmed",
  "created_at": "2024-01-15T10:00:00Z",
  "updated_at": "2024-01-15T10:05:00Z",
  "expires_at": null,
  "slot": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "start_time": "2024-01-20T14:00:00Z",
    "end_time": "2024-01-20T15:00:00Z",
    "status": "open",
    "capacity": 1
  },
  "patient": {
    "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
    "name": "John Doe",
    "email": "john@example.com"
  },
  "clinician": {
    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "name": "Dr. Jane Smith",
    "specialty": "Cardiology"
  }
}
```

**GET `/appointments?patient_id={uuid}`**
List appointments for a specific patient.

Query Parameters:

- `patient_id` (required) - UUID of the patient
- `limit` (optional, default: 20, max: 100) - Number of results
- `offset` (optional, default: 0) - Pagination offset

**GET `/appointments?slot_id={uuid}`**
List appointments for a specific slot.

Query Parameters:

- `slot_id` (required) - UUID of the slot

### Error Response Format

All errors follow this structure:

```json
{
  "error": "error_code",
  "details": "Human-readable error message"
}
```

## The Simulator Tool

The simulator (`cmd/simulate`) is a load testing tool that generates realistic traffic patterns against your API to validate system behavior under contention.

### What It Does

- Loads real patient and slot IDs from your database
- Generates concurrent HTTP requests following configurable ratios
- Measures performance metrics (latency, success rates, conflicts)
- Provides detailed reports on system behavior

### Why Use It

- **Validate Correctness**: Ensure no double-booking occurs under load
- **Performance Testing**: Measure response times and throughput
- **Contention Analysis**: See how the system handles concurrent bookings
- **Regression Testing**: Compare behavior before/after code changes

### Usage

Basic usage:

```bash
go run ./cmd/simulate
```

With custom parameters:

```bash
SIM_DURATION=2m \
SIM_WORKERS=50 \
SIM_BOOKING_RATIO=0.6 \
SIM_CONFIRM_RATIO=0.2 \
SIM_READ_RATIO=0.2 \
go run ./cmd/simulate
```

### Configuration

Environment variables:

- `SIM_API_BASE_URL` - API endpoint (default: `http://localhost:8080`)
- `SIM_DURATION` - How long to run (default: `30s`)
- `SIM_WORKERS` - Number of concurrent workers (default: `10`)
- `SIM_BOOKING_RATIO` - Percentage for booking operations (default: `0.5`)
- `SIM_CONFIRM_RATIO` - Percentage for confirm operations (default: `0.2`)
- `SIM_READ_RATIO` - Percentage for read operations (default: `0.3`)
- `SIM_PATIENT_LIMIT` - Max patients to load (default: `4000`)
- `SIM_SLOT_LIMIT` - Max slots to load (default: `2400`)

**Note**: Ratios are automatically normalized if they don't sum to 1.0.

### Sample Output

```
================================================================================
SIMULATION REPORT
================================================================================
Duration: 1m0s
Workers: 50

Booking:
  Total: 5000
  Success: 3000 (60.0%)
  Conflicts: 1800 (36.0%)
  Latency: avg=40ms min=5ms max=250ms p50=35ms p95=120ms

Confirm:
  Total: 2000
  Success: 1500 (75.0%)
  Conflicts: 450 (22.5%)
  Latency: avg=25ms min=3ms max=180ms p50=20ms p95=60ms

Read by ID:
  Total: 4000
  Success: 4000 (100.0%)
  Latency: avg=10ms min=2ms max=50ms p50=8ms p95=30ms
```

## Database Schema

### Core Tables

- **`patients`** - Patient information
- **`clinicians`** - Healthcare provider information
- **`appointment_slots`** - Available time slots
- **`appointments`** - Appointment records with status
- **`event_logs`** - Audit trail of all state changes

### Key Constraints

1. **Partial Unique Index**: Only one confirmed appointment per slot

   ```sql
   CREATE UNIQUE INDEX uniq_confirmed_appointment_per_slot
       ON appointments (slot_id) WHERE status = 'confirmed';
   ```

2. **Time Range Validation**: Slots must have valid time ranges
3. **Foreign Key Constraints**: Referential integrity across tables
4. **Status Enums**: Type-safe status values

### Migrations

Migrations are located in `internal/db/migrations/`:

1. `0001_init_core.sql` - Core tables and enums
2. `0002_slots_appointments.sql` - Slots and appointments with constraints
3. `0003_event_logs.sql` - Event logging table
4. `0004_read_indexes.sql` - Performance indexes for read queries

Run migrations in order before starting the application.

## How It Works: The Booking Flow

Understanding how the system prevents double-booking:

1. **Client Request**: User attempts to book a slot
2. **Validation**: System checks patient exists and slot is open
3. **Distributed Lock**: Acquires Redis lock for the specific slot
4. **Double-Check**: Inside the lock, verifies no confirmed appointment exists
5. **Create Pending**: Creates appointment with `pending` status and expiry time
6. **Release Lock**: Releases Redis lock
7. **Event Logging**: Records `APPOINTMENT_CREATED` event

If another request tries to book the same slot:

- It will either fail to acquire the lock (returns `slot_being_booked`)
- Or acquire the lock but find an existing confirmed appointment (returns `slot_already_booked`)

The database constraint provides a final safety net: even if two requests somehow both create appointments, only one can be confirmed due to the partial unique index.

## Development

### Project Structure

```
.
├── cmd/                    # Application entry points
│   ├── api-server/         # HTTP API server
│   ├── expiry-worker/      # Background expiry worker
│   ├── seed/               # Database seeding tool
│   └── simulate/           # Load testing simulator
├── internal/               # Private application code
│   ├── api/                # HTTP handlers and routing
│   ├── appointment/        # Domain logic and repository
│   ├── config/             # Configuration management
│   ├── db/                 # Database connection and migrations
│   └── redis/              # Redis client and locking
└── go.mod                  # Go module definition
```

### Running Tests

```bash
go test ./...
```

### Code Style

The project follows standard Go conventions:

- Use `gofmt` for formatting
- Follow effective Go guidelines
- Keep functions focused and testable

## Production Considerations

### Deployment

1. **Database**: Use managed PostgreSQL with connection pooling
2. **Redis**: Use managed Redis or Redis Cluster for high availability
3. **API Server**: Deploy multiple instances behind a load balancer
4. **Expiry Worker**: Run one instance per environment (or use leader election)

### Monitoring

- Use health endpoints (`/health/live`, `/health/ready`) for orchestration
- Monitor Redis lock acquisition failures
- Track appointment creation/confirmation rates
- Alert on high error rates or latency spikes

### Scaling

- **Horizontal Scaling**: API servers are stateless and can scale horizontally
- **Database**: Use read replicas for read-heavy workloads
- **Redis**: Use Redis Cluster for distributed locking across regions

### Security

- Use TLS for all HTTP traffic
- Secure Redis with authentication
- Use connection string encryption for database credentials
- Implement rate limiting for API endpoints
- Add authentication/authorization middleware

## Troubleshooting

### Common Issues

**"POSTGRES_DSN is required"**

- Ensure `.env` file exists with `POSTGRES_DSN` set
- Or set `POSTGRES_DSN` environment variable

**"slot_being_booked" errors**

- This is expected under high concurrency
- Clients should retry with exponential backoff
- Consider increasing `LOCK_TTL` if locks expire too quickly

**High latency on bookings**

- Check Redis connection and latency
- Verify database indexes are being used
- Monitor connection pool usage

**Appointments not expiring**

- Verify expiry worker is running
- Check `WORKER_INTERVAL` configuration
- Verify `APPOINTMENT_TTL` is set correctly