# Shipment Tracking gRPC Microservice

A gRPC microservice for managing shipments and tracking shipment status changes during transportation.

## How to Run the Service

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- [golang-migrate CLI](https://github.com/golang-migrate/migrate) (for local migrations)
- [buf](https://buf.build/docs/installation) (for proto generation/linting)
- [golangci-lint](https://golangci-lint.run/welcome/install/) (for code linting)

### Run with Docker Compose (recommended)
```bash
make up
# This starts PostgreSQL, runs migrations, and starts the gRPC server.
```

### Run locally
```bash
# Start PostgreSQL (e.g. via Docker)
docker run -d --name shipment-pg -p 5432:5432 \
  -e POSTGRES_USER=shipment -e POSTGRES_PASSWORD=shipment -e POSTGRES_DB=shipment \
  postgres:16-alpine

# Apply migrations
DATABASE_URL="postgres://shipment:shipment@localhost:5432/shipment?sslmode=disable" make migrate-up

# Run the server
make run
```

The gRPC server starts on port `50051` by default.

### Configuration

Configuration is loaded from `config/config.yaml` with environment variable overrides.

| Variable | Config key | Default | Description |
|---|---|---|---|
| `GRPC_PORT` | `grpc.port` | `50051` | gRPC server listen port |
| `GRPC_SHUTDOWN_TIMEOUT` | `grpc.shutdown_timeout` | `10` | Graceful shutdown timeout (seconds) |
| `DATABASE_URL` | `database.url` | — | PostgreSQL connection string |
| `MIGRATIONS_PATH` | `database.migrations_path` | `migrations` | Path to migration files |
| `LOG_LEVEL` | `log.level` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `LOG_FORMAT` | `log.format` | `json` | Log format (`json` or `console`) |

### Available Make Targets

| Target | Description |
|---|---|
| `make build` | Build the server binary |
| `make run` | Build and run the server |
| `make test` | Run all tests (unit + integration) |
| `make lint` | Run buf lint and golangci-lint |
| `make proto` | Regenerate protobuf code via buf |
| `make migrate-up` | Apply database migrations (requires `DATABASE_URL`) |
| `make migrate-down` | Roll back last migration (requires `DATABASE_URL`) |
| `make up` | Start all services via Docker Compose |
| `make down` | Stop and remove all containers and volumes |

## How to Run the Tests

```bash
make test
# or:
go test ./... -v
```

**Note:** Integration tests require Docker to be running (uses [testcontainers-go](https://golang.testcontainers.org/) to spin up PostgreSQL). Use `-short` flag to skip them:
```bash
go test ./... -short
```

## Architecture Overview

The project follows **Clean Architecture** principles with clear separation of concerns:

```
├── config/
│   └── config.yaml                 # Application configuration
├── migrations/                     # SQL migration files (golang-migrate)
├── proto/                          # Protocol Buffer definitions
│   └── shipment/v1/
│       └── shipment.proto
├── gen/                            # Generated protobuf Go code
│   └── shipment/v1/
├── cmd/
│   └── server/
│       └── main.go                 # Entry point, wiring
├── internal/
│   ├── config/                     # Configuration loading (viper)
│   │   └── config.go
│   ├── logger/                     # Logger setup (zerolog)
│   │   └── logger.go
│   ├── domain/                     # Domain layer (entities, business rules)
│   │   ├── shipment.go             # Shipment entity and StatusEvent
│   │   ├── status.go               # Status type and transition rules
│   │   ├── errors.go               # Domain errors
│   │   ├── repository.go           # Repository + TxManager interfaces
│   │   └── shipment_test.go        # Domain logic tests
│   ├── application/                # Application layer (use cases)
│   │   ├── service.go              # ShipmentService use cases
│   │   └── service_test.go         # Integration tests (testcontainers)
│   ├── infrastructure/             # Infrastructure layer
│   │   └── postgres/
│   │       ├── shipment_repo.go    # PostgreSQL shipment repository
│   │       ├── event_repo.go       # PostgreSQL status event repository
│   │       ├── tx.go               # Transaction manager
│   │       └── migrate.go          # Migration helper (used by tests)
│   └── transport/                  # Transport layer
│       └── grpc/
│           ├── handler.go          # gRPC handler (maps proto <-> domain)
│           └── interceptor.go      # Request ID + logging interceptors
├── .golangci.yml                   # Linter configuration
├── buf.yaml                        # Buf module configuration
├── buf.gen.yaml                    # Buf code generation config
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

### Layer Responsibilities

- **Domain Layer** (`internal/domain/`): Contains the core business entities (`Shipment`, `StatusEvent`), value objects (`Status`), business rules (status transition validation), domain errors, and repository/transaction interfaces. This layer has **zero external dependencies** — no gRPC, no database, no framework imports.

- **Application Layer** (`internal/application/`): Orchestrates use cases (create shipment, add status event, list shipments, etc.) by coordinating domain entities and repository interfaces. Uses database transactions for multi-step operations. Includes structured logging for all operations.

- **Infrastructure Layer** (`internal/infrastructure/`): Implements the repository interfaces defined in the domain. Provides PostgreSQL implementations using `pgx`, including a transaction manager. Database migrations are managed with `golang-migrate` CLI.

- **Transport Layer** (`internal/transport/grpc/`): Maps between protobuf messages and domain types. Handles gRPC-specific concerns (error codes, request validation). Includes interceptors for request ID injection and structured logging. Registers the standard gRPC health check service.

## Design Decisions

### Status Lifecycle

Shipments follow a strict state machine:

```
pending → picked_up → in_transit → delivered
   ↓          ↓            ↓
cancelled  cancelled    cancelled
```

- **`pending`**: Initial status when a shipment is created.
- **`picked_up`**: Driver has picked up the shipment.
- **`in_transit`**: Shipment is on the road.
- **`delivered`**: Shipment has been delivered (terminal state).
- **`cancelled`**: Shipment was cancelled (terminal state). Can be cancelled from any non-terminal state.

### Key Rules
- A shipment always starts as `pending`.
- Transitions must follow the defined state machine — invalid transitions are rejected.
- Duplicate status updates (transitioning to the current status) are rejected.
- `delivered` and `cancelled` are terminal states — no further transitions allowed.
- Every status change is recorded as a `StatusEvent` with a timestamp and optional comment.
- The shipment's current status always reflects the latest valid event.

### Money Representation
Monetary values (`shipment_amount`, `driver_revenue`) are stored as **cents (int64)** to avoid floating-point precision issues.

### Repository Interfaces in Domain
Repository interfaces are defined in the domain layer (ports), while implementations live in infrastructure (adapters). This allows the domain to be tested independently.

### Database Transactions
Multi-step write operations (`CreateShipment`, `AddStatusEvent`) are wrapped in database transactions via a `TxManager` interface. The repos detect an active transaction from context and participate automatically. This ensures atomic consistency — e.g. a status event and shipment update either both succeed or both roll back.

### Request ID
Every gRPC request gets a unique UUID injected via interceptor. The request ID is included in all log entries for the request, enabling end-to-end tracing.

### Health Check
The standard `grpc.health.v1.Health` service is registered, supporting readiness probes from k8s/load balancers. The serving status is set to `NOT_SERVING` during shutdown to allow graceful draining.

### Graceful Shutdown
On SIGINT/SIGTERM, the server marks itself as not serving, then attempts a graceful stop with a configurable timeout (default 10s). If in-flight requests don't complete in time, the server is forcefully stopped.

### Logging
Structured logging is implemented using `zerolog`:
- **Application layer**: Logs shipment creation, status transitions (from/to), errors, and history retrieval.
- **gRPC interceptor**: Logs every request with method name, request ID, duration, gRPC status code, and errors. Log level is adjusted based on the status code (info for success, warn for client errors, error for server errors).
- **Configuration**: Log level and format (JSON for production, console for development) are configurable via `config.yaml` or environment variables.

### Migrations
Database migrations are managed using the `golang-migrate` CLI — no custom migration code in the server. In Docker Compose, migrations run as a separate init service before the server starts.

### Proto Management
Protocol Buffer definitions are managed with [buf](https://buf.build). `buf lint` enforces naming conventions and best practices. `buf generate` replaces raw `protoc` commands for code generation.

### CI
GitHub Actions workflow runs lint, build, unit tests, integration tests, and proto lint on every push/PR.

## Assumptions

1. **Authentication/Authorization**: Not implemented. In production, this would be handled via gRPC interceptors with JWT or mTLS.
2. **Persistence**: Uses PostgreSQL with `pgx` driver.
3. **Concurrency**: PostgreSQL handles transactional consistency.
4. **Event Sourcing**: While status changes are recorded as events, this is not a full event-sourced system. The shipment entity holds the current state directly.
5. **gRPC Reflection**: Enabled for development convenience (allows tools like `grpcurl` to discover the API).
6. **Testing**: Integration tests use `testcontainers-go` to spin up real PostgreSQL instances. Domain tests are pure unit tests with no external dependencies.

## API Reference

| RPC Method | Description |
|---|---|
| `CreateShipment` | Create a new shipment (starts as `pending`) |
| `GetShipment` | Retrieve shipment details by ID |
| `ListShipments` | List shipments with pagination (`limit`/`offset`) |
| `AddStatusEvent` | Add a status change event to a shipment |
| `GetShipmentHistory` | Get all status events for a shipment |
| `grpc.health.v1.Health/Check` | Standard gRPC health check |

### Example with grpcurl

```bash
# Create a shipment
grpcurl -plaintext -d '{
  "reference_number": "SH-001",
  "origin": "New York",
  "destination": "Los Angeles",
  "driver": {"name": "John", "phone": "555-1234"},
  "unit": {"unit_number": "TRUCK-01", "unit_type": "Flatbed"},
  "shipment_amount_cents": 150000,
  "driver_revenue_cents": 75000
}' localhost:50051 shipment.v1.ShipmentService/CreateShipment

# Get shipment
grpcurl -plaintext -d '{"id": "<shipment-id>"}' localhost:50051 shipment.v1.ShipmentService/GetShipment

# List shipments
grpcurl -plaintext -d '{"limit": 10, "offset": 0}' localhost:50051 shipment.v1.ShipmentService/ListShipments

# Add status event
grpcurl -plaintext -d '{
  "shipment_id": "<shipment-id>",
  "status": "SHIPMENT_STATUS_PICKED_UP",
  "comment": "Driver arrived at origin"
}' localhost:50051 shipment.v1.ShipmentService/AddStatusEvent

# Get history
grpcurl -plaintext -d '{"shipment_id": "<shipment-id>"}' localhost:50051 shipment.v1.ShipmentService/GetShipmentHistory

# Health check
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
```
