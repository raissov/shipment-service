package domain_test

import (
	"testing"

	"github.com/raissov/shipment-service/internal/domain"
)

func TestNewShipment(t *testing.T) {
	t.Run("creates shipment with pending status", func(t *testing.T) {
		s, err := domain.NewShipment(
			"REF-001", "New York", "Los Angeles",
			domain.Driver{Name: "John", Phone: "555-1234"},
			domain.Unit{UnitNumber: "UNIT-01", UnitType: "Truck"},
			100000, 50000,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Status != domain.StatusPending {
			t.Errorf("expected status %q, got %q", domain.StatusPending, s.Status)
		}
		if s.ReferenceNumber != "REF-001" {
			t.Errorf("expected ref REF-001, got %s", s.ReferenceNumber)
		}
		if s.ID == "" {
			t.Error("expected non-empty ID")
		}
	})

	t.Run("fails without reference number", func(t *testing.T) {
		_, err := domain.NewShipment("", "A", "B", domain.Driver{}, domain.Unit{}, 0, 0)
		if err == nil {
			t.Error("expected error for empty reference number")
		}
	})

	t.Run("fails without origin", func(t *testing.T) {
		_, err := domain.NewShipment("REF", "", "B", domain.Driver{}, domain.Unit{}, 0, 0)
		if err == nil {
			t.Error("expected error for empty origin")
		}
	})

	t.Run("fails without destination", func(t *testing.T) {
		_, err := domain.NewShipment("REF", "A", "", domain.Driver{}, domain.Unit{}, 0, 0)
		if err == nil {
			t.Error("expected error for empty destination")
		}
	})
}

func TestShipment_AddStatusEvent(t *testing.T) {
	newShipment := func() *domain.Shipment {
		s, _ := domain.NewShipment(
			"REF-001", "Origin", "Destination",
			domain.Driver{Name: "Driver", Phone: "123"},
			domain.Unit{UnitNumber: "U1", UnitType: "Truck"},
			10000, 5000,
		)
		return s
	}

	t.Run("valid transition pending -> picked_up", func(t *testing.T) {
		s := newShipment()
		event, err := s.AddStatusEvent(domain.StatusPickedUp, "Picked up by driver")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Status != domain.StatusPickedUp {
			t.Errorf("expected status %q, got %q", domain.StatusPickedUp, s.Status)
		}
		if event.Status != domain.StatusPickedUp {
			t.Errorf("event status should be picked_up, got %q", event.Status)
		}
		if event.Comment != "Picked up by driver" {
			t.Errorf("unexpected comment: %s", event.Comment)
		}
	})

	t.Run("valid transition picked_up -> in_transit", func(t *testing.T) {
		s := newShipment()
		_, _ = s.AddStatusEvent(domain.StatusPickedUp, "")
		_, err := s.AddStatusEvent(domain.StatusInTransit, "On the road")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Status != domain.StatusInTransit {
			t.Errorf("expected in_transit, got %q", s.Status)
		}
	})

	t.Run("valid transition in_transit -> delivered", func(t *testing.T) {
		s := newShipment()
		_, _ = s.AddStatusEvent(domain.StatusPickedUp, "")
		_, _ = s.AddStatusEvent(domain.StatusInTransit, "")
		_, err := s.AddStatusEvent(domain.StatusDelivered, "Delivered to recipient")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Status != domain.StatusDelivered {
			t.Errorf("expected delivered, got %q", s.Status)
		}
	})

	t.Run("valid transition pending -> cancelled", func(t *testing.T) {
		s := newShipment()
		_, err := s.AddStatusEvent(domain.StatusCancelled, "Customer request")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Status != domain.StatusCancelled {
			t.Errorf("expected cancelled, got %q", s.Status)
		}
	})

	t.Run("invalid transition pending -> delivered", func(t *testing.T) {
		s := newShipment()
		_, err := s.AddStatusEvent(domain.StatusDelivered, "")
		if err == nil {
			t.Error("expected error for invalid transition")
		}
		if s.Status != domain.StatusPending {
			t.Errorf("status should remain pending, got %q", s.Status)
		}
	})

	t.Run("invalid transition pending -> in_transit", func(t *testing.T) {
		s := newShipment()
		_, err := s.AddStatusEvent(domain.StatusInTransit, "")
		if err == nil {
			t.Error("expected error for invalid transition")
		}
	})

	t.Run("invalid transition delivered -> any", func(t *testing.T) {
		s := newShipment()
		_, _ = s.AddStatusEvent(domain.StatusPickedUp, "")
		_, _ = s.AddStatusEvent(domain.StatusInTransit, "")
		_, _ = s.AddStatusEvent(domain.StatusDelivered, "")

		_, err := s.AddStatusEvent(domain.StatusInTransit, "")
		if err == nil {
			t.Error("expected error: delivered is a terminal state")
		}
	})

	t.Run("invalid transition cancelled -> any", func(t *testing.T) {
		s := newShipment()
		_, _ = s.AddStatusEvent(domain.StatusCancelled, "")

		_, err := s.AddStatusEvent(domain.StatusPickedUp, "")
		if err == nil {
			t.Error("expected error: cancelled is a terminal state")
		}
	})

	t.Run("duplicate status rejected", func(t *testing.T) {
		s := newShipment()
		_, err := s.AddStatusEvent(domain.StatusPending, "")
		if err == nil {
			t.Error("expected error for duplicate status")
		}
	})

	t.Run("invalid status value rejected", func(t *testing.T) {
		s := newShipment()
		_, err := s.AddStatusEvent(domain.Status("unknown"), "")
		if err == nil {
			t.Error("expected error for invalid status")
		}
	})
}

func TestStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from    domain.Status
		to      domain.Status
		allowed bool
	}{
		{domain.StatusPending, domain.StatusPickedUp, true},
		{domain.StatusPending, domain.StatusCancelled, true},
		{domain.StatusPending, domain.StatusInTransit, false},
		{domain.StatusPending, domain.StatusDelivered, false},
		{domain.StatusPickedUp, domain.StatusInTransit, true},
		{domain.StatusPickedUp, domain.StatusCancelled, true},
		{domain.StatusPickedUp, domain.StatusDelivered, false},
		{domain.StatusInTransit, domain.StatusDelivered, true},
		{domain.StatusInTransit, domain.StatusCancelled, true},
		{domain.StatusInTransit, domain.StatusPickedUp, false},
		{domain.StatusDelivered, domain.StatusCancelled, false},
		{domain.StatusDelivered, domain.StatusPending, false},
		{domain.StatusCancelled, domain.StatusPending, false},
	}

	for _, tt := range tests {
		name := string(tt.from) + " -> " + string(tt.to)
		t.Run(name, func(t *testing.T) {
			got := tt.from.CanTransitionTo(tt.to)
			if got != tt.allowed {
				t.Errorf("expected %v, got %v", tt.allowed, got)
			}
		})
	}
}

func TestStatus_IsValid(t *testing.T) {
	if !domain.StatusPending.IsValid() {
		t.Error("pending should be valid")
	}
	if domain.Status("bogus").IsValid() {
		t.Error("bogus should be invalid")
	}
}
