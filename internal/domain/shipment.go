package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Shipment is the core domain entity representing a logistics shipment.
type Shipment struct {
	ID                  string
	ReferenceNumber     string
	Origin              string
	Destination         string
	Status              Status
	Driver              Driver
	Unit                Unit
	ShipmentAmountCents int64
	DriverRevenueCents  int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// Driver holds driver information.
type Driver struct {
	Name  string
	Phone string
}

// Unit holds transport unit information.
type Unit struct {
	UnitNumber string
	UnitType   string
}

// StatusEvent represents a recorded change in shipment status.
type StatusEvent struct {
	ID         string
	ShipmentID string
	Status     Status
	Comment    string
	OccurredAt time.Time
}

// NewShipment creates a new shipment with an initial pending status.
func NewShipment(
	referenceNumber, origin, destination string,
	driver Driver,
	unit Unit,
	shipmentAmountCents, driverRevenueCents int64,
) (*Shipment, error) {
	if referenceNumber == "" {
		return nil, fmt.Errorf("reference number is required")
	}
	if origin == "" {
		return nil, fmt.Errorf("origin is required")
	}
	if destination == "" {
		return nil, fmt.Errorf("destination is required")
	}

	now := time.Now().UTC()
	return &Shipment{
		ID:                  uuid.New().String(),
		ReferenceNumber:     referenceNumber,
		Origin:              origin,
		Destination:         destination,
		Status:              StatusPending,
		Driver:              driver,
		Unit:                unit,
		ShipmentAmountCents: shipmentAmountCents,
		DriverRevenueCents:  driverRevenueCents,
		CreatedAt:           now,
		UpdatedAt:           now,
	}, nil
}

// AddStatusEvent validates and applies a status transition, returning the new event.
func (s *Shipment) AddStatusEvent(newStatus Status, comment string) (*StatusEvent, error) {
	if !newStatus.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidStatus, newStatus)
	}

	if s.Status == newStatus {
		return nil, fmt.Errorf("%w: status is already %q", ErrDuplicateStatus, newStatus)
	}

	if err := s.Status.ValidateTransition(newStatus); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	s.Status = newStatus
	s.UpdatedAt = now

	event := &StatusEvent{
		ID:         uuid.New().String(),
		ShipmentID: s.ID,
		Status:     newStatus,
		Comment:    comment,
		OccurredAt: now,
	}

	return event, nil
}
