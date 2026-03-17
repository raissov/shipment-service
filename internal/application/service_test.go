package application_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/raissov/shipment-service/internal/application"
	"github.com/raissov/shipment-service/internal/domain"
	"github.com/raissov/shipment-service/internal/infrastructure/postgres"
)

func setupTestService(t *testing.T) *application.ShipmentService {
	t.Helper()
	ctx := context.Background()

	_, currentFile, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "migrations")

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("shipment_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	// Run migrations
	if err := postgres.MigrateUp(connStr, migrationsPath); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	return application.NewShipmentService(
		postgres.NewShipmentRepository(pool),
		postgres.NewStatusEventRepository(pool),
		postgres.NewTxManager(pool),
		zerolog.Nop(),
	)
}

func createTestShipment(t *testing.T, svc *application.ShipmentService) *domain.Shipment {
	t.Helper()
	s, err := svc.CreateShipment(context.Background(), application.CreateShipmentInput{
		ReferenceNumber:     "REF-001",
		Origin:              "New York",
		Destination:         "Los Angeles",
		DriverName:          "John",
		DriverPhone:         "555-1234",
		UnitNumber:          "UNIT-01",
		UnitType:            "Truck",
		ShipmentAmountCents: 100000,
		DriverRevenueCents:  50000,
	})
	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}
	return s
}

func TestCreateShipment(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	svc := setupTestService(t)

	t.Run("creates shipment successfully", func(t *testing.T) {
		s := createTestShipment(t, svc)
		if s.Status != domain.StatusPending {
			t.Errorf("expected pending, got %q", s.Status)
		}
		if s.ReferenceNumber != "REF-001" {
			t.Errorf("expected REF-001, got %s", s.ReferenceNumber)
		}
	})

	t.Run("fails with invalid input", func(t *testing.T) {
		_, err := svc.CreateShipment(context.Background(), application.CreateShipmentInput{})
		if err == nil {
			t.Error("expected error for empty input")
		}
	})
}

func TestGetShipment(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	svc := setupTestService(t)

	t.Run("retrieves existing shipment", func(t *testing.T) {
		created := createTestShipment(t, svc)
		found, err := svc.GetShipment(context.Background(), created.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found.ID != created.ID {
			t.Errorf("expected ID %s, got %s", created.ID, found.ID)
		}
	})

	t.Run("returns error for non-existent shipment", func(t *testing.T) {
		_, err := svc.GetShipment(context.Background(), "00000000-0000-0000-0000-000000000000")
		if err == nil {
			t.Error("expected error")
		}
		if !errors.Is(err, domain.ErrShipmentNotFound) {
			t.Errorf("expected ErrShipmentNotFound, got: %v", err)
		}
	})
}

func TestAddStatusEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	svc := setupTestService(t)

	t.Run("valid transition updates shipment", func(t *testing.T) {
		s := createTestShipment(t, svc)

		event, shipment, err := svc.AddStatusEvent(context.Background(), s.ID, domain.StatusPickedUp, "Driver arrived")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if event.Status != domain.StatusPickedUp {
			t.Errorf("expected event status picked_up, got %q", event.Status)
		}
		if shipment.Status != domain.StatusPickedUp {
			t.Errorf("expected shipment status picked_up, got %q", shipment.Status)
		}

		// Verify persistence
		found, _ := svc.GetShipment(context.Background(), s.ID)
		if found.Status != domain.StatusPickedUp {
			t.Errorf("persisted status should be picked_up, got %q", found.Status)
		}
	})

	t.Run("invalid transition rejected", func(t *testing.T) {
		s := createTestShipment(t, svc)

		_, _, err := svc.AddStatusEvent(context.Background(), s.ID, domain.StatusDelivered, "")
		if err == nil {
			t.Error("expected error for invalid transition")
		}
		if !errors.Is(err, domain.ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got: %v", err)
		}

		// Status should remain unchanged
		found, _ := svc.GetShipment(context.Background(), s.ID)
		if found.Status != domain.StatusPending {
			t.Errorf("status should remain pending, got %q", found.Status)
		}
	})

	t.Run("full lifecycle", func(t *testing.T) {
		s := createTestShipment(t, svc)

		transitions := []domain.Status{
			domain.StatusPickedUp,
			domain.StatusInTransit,
			domain.StatusDelivered,
		}
		for _, status := range transitions {
			_, _, err := svc.AddStatusEvent(context.Background(), s.ID, status, "")
			if err != nil {
				t.Fatalf("transition to %q failed: %v", status, err)
			}
		}

		found, _ := svc.GetShipment(context.Background(), s.ID)
		if found.Status != domain.StatusDelivered {
			t.Errorf("expected delivered, got %q", found.Status)
		}
	})

	t.Run("non-existent shipment", func(t *testing.T) {
		_, _, err := svc.AddStatusEvent(context.Background(), "00000000-0000-0000-0000-000000000000", domain.StatusPickedUp, "")
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestGetShipmentHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	svc := setupTestService(t)
	s := createTestShipment(t, svc)

	_, _, _ = svc.AddStatusEvent(context.Background(), s.ID, domain.StatusPickedUp, "Picked up")
	_, _, _ = svc.AddStatusEvent(context.Background(), s.ID, domain.StatusInTransit, "On the way")

	events, err := svc.GetShipmentHistory(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 events: initial pending + picked_up + in_transit
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	expectedStatuses := []domain.Status{domain.StatusPending, domain.StatusPickedUp, domain.StatusInTransit}
	for i, expected := range expectedStatuses {
		if events[i].Status != expected {
			t.Errorf("event[%d]: expected %q, got %q", i, expected, events[i].Status)
		}
	}
}

func TestListShipments(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	svc := setupTestService(t)

	// Create a few shipments
	for i := 0; i < 3; i++ {
		_, err := svc.CreateShipment(context.Background(), application.CreateShipmentInput{
			ReferenceNumber:     fmt.Sprintf("REF-%03d", i),
			Origin:              "Origin",
			Destination:         "Destination",
			ShipmentAmountCents: 10000,
			DriverRevenueCents:  5000,
		})
		if err != nil {
			t.Fatalf("failed to create shipment: %v", err)
		}
	}

	t.Run("returns all shipments", func(t *testing.T) {
		shipments, total, err := svc.ListShipments(context.Background(), 10, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 3 {
			t.Errorf("expected total 3, got %d", total)
		}
		if len(shipments) != 3 {
			t.Errorf("expected 3 shipments, got %d", len(shipments))
		}
	})

	t.Run("respects pagination", func(t *testing.T) {
		shipments, total, err := svc.ListShipments(context.Background(), 2, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 3 {
			t.Errorf("expected total 3, got %d", total)
		}
		if len(shipments) != 2 {
			t.Errorf("expected 2 shipments, got %d", len(shipments))
		}
	})

	t.Run("offset beyond total returns empty", func(t *testing.T) {
		shipments, total, err := svc.ListShipments(context.Background(), 10, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 3 {
			t.Errorf("expected total 3, got %d", total)
		}
		if len(shipments) != 0 {
			t.Errorf("expected 0 shipments, got %d", len(shipments))
		}
	})
}

func TestGetShipmentHistory_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	svc := setupTestService(t)
	_, err := svc.GetShipmentHistory(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Error("expected error for non-existent shipment")
	}
}
