package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/raissov/shipment-service/internal/domain"
)

// ShipmentService implements the application use cases for shipment management.
type ShipmentService struct {
	shipmentRepo domain.ShipmentRepository
	eventRepo    domain.StatusEventRepository
	txManager    domain.TxManager
	log          zerolog.Logger
}

// NewShipmentService creates a new ShipmentService with the given dependencies.
func NewShipmentService(
	shipmentRepo domain.ShipmentRepository,
	eventRepo domain.StatusEventRepository,
	txManager domain.TxManager,
	log zerolog.Logger,
) *ShipmentService {
	return &ShipmentService{
		shipmentRepo: shipmentRepo,
		eventRepo:    eventRepo,
		txManager:    txManager,
		log:          log.With().Str("component", "shipment_service").Logger(),
	}
}

// CreateShipmentInput holds the input data for creating a shipment.
type CreateShipmentInput struct {
	ReferenceNumber     string
	Origin              string
	Destination         string
	DriverName          string
	DriverPhone         string
	UnitNumber          string
	UnitType            string
	ShipmentAmountCents int64
	DriverRevenueCents  int64
}

// CreateShipment creates a new shipment and persists it.
func (s *ShipmentService) CreateShipment(ctx context.Context, input CreateShipmentInput) (*domain.Shipment, error) {
	shipment, err := domain.NewShipment(
		input.ReferenceNumber,
		input.Origin,
		input.Destination,
		domain.Driver{Name: input.DriverName, Phone: input.DriverPhone},
		domain.Unit{UnitNumber: input.UnitNumber, UnitType: input.UnitType},
		input.ShipmentAmountCents,
		input.DriverRevenueCents,
	)
	if err != nil {
		s.log.Warn().Err(err).Str("reference", input.ReferenceNumber).Msg("failed to create shipment entity")
		return nil, fmt.Errorf("creating shipment: %w", err)
	}

	initialEvent := &domain.StatusEvent{
		ID:         uuid.New().String(),
		ShipmentID: shipment.ID,
		Status:     domain.StatusPending,
		Comment:    "Shipment created",
		OccurredAt: shipment.CreatedAt,
	}

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.shipmentRepo.Save(txCtx, shipment); err != nil {
			return fmt.Errorf("saving shipment: %w", err)
		}
		if err := s.eventRepo.Save(txCtx, initialEvent); err != nil {
			return fmt.Errorf("saving initial event: %w", err)
		}
		return nil
	})
	if err != nil {
		s.log.Error().Err(err).Str("shipment_id", shipment.ID).Msg("failed to create shipment")
		return nil, err
	}

	s.log.Info().
		Str("shipment_id", shipment.ID).
		Str("reference", shipment.ReferenceNumber).
		Str("origin", shipment.Origin).
		Str("destination", shipment.Destination).
		Msg("shipment created")

	return shipment, nil
}

// GetShipment retrieves a shipment by ID.
func (s *ShipmentService) GetShipment(ctx context.Context, id string) (*domain.Shipment, error) {
	shipment, err := s.shipmentRepo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn().Err(err).Str("shipment_id", id).Msg("failed to find shipment")
		return nil, fmt.Errorf("finding shipment: %w", err)
	}
	return shipment, nil
}

// ListShipments retrieves a paginated list of shipments.
func (s *ShipmentService) ListShipments(ctx context.Context, limit, offset int) ([]*domain.Shipment, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	shipments, total, err := s.shipmentRepo.List(ctx, limit, offset)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to list shipments")
		return nil, 0, fmt.Errorf("listing shipments: %w", err)
	}

	s.log.Debug().Int("count", len(shipments)).Int("total", total).Msg("shipments listed")
	return shipments, total, nil
}

// AddStatusEvent adds a new status event to a shipment, enforcing business rules.
func (s *ShipmentService) AddStatusEvent(ctx context.Context, shipmentID string, status domain.Status, comment string) (*domain.StatusEvent, *domain.Shipment, error) {
	shipment, err := s.shipmentRepo.FindByID(ctx, shipmentID)
	if err != nil {
		s.log.Warn().Err(err).Str("shipment_id", shipmentID).Msg("failed to find shipment for status update")
		return nil, nil, fmt.Errorf("finding shipment: %w", err)
	}

	previousStatus := shipment.Status
	event, err := shipment.AddStatusEvent(status, comment)
	if err != nil {
		s.log.Warn().Err(err).
			Str("shipment_id", shipmentID).
			Str("from_status", string(previousStatus)).
			Str("to_status", string(status)).
			Msg("invalid status transition")
		return nil, nil, err
	}

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.eventRepo.Save(txCtx, event); err != nil {
			return fmt.Errorf("saving event: %w", err)
		}
		if err := s.shipmentRepo.Update(txCtx, shipment); err != nil {
			return fmt.Errorf("updating shipment: %w", err)
		}
		return nil
	})
	if err != nil {
		s.log.Error().Err(err).Str("shipment_id", shipmentID).Msg("failed to persist status transition")
		return nil, nil, err
	}

	s.log.Info().
		Str("shipment_id", shipmentID).
		Str("from_status", string(previousStatus)).
		Str("to_status", string(status)).
		Str("event_id", event.ID).
		Msg("status transition applied")

	return event, shipment, nil
}

// GetShipmentHistory retrieves all status events for a shipment.
func (s *ShipmentService) GetShipmentHistory(ctx context.Context, shipmentID string) ([]*domain.StatusEvent, error) {
	if _, err := s.shipmentRepo.FindByID(ctx, shipmentID); err != nil {
		s.log.Warn().Err(err).Str("shipment_id", shipmentID).Msg("failed to find shipment for history")
		return nil, fmt.Errorf("finding shipment: %w", err)
	}

	events, err := s.eventRepo.FindByShipmentID(ctx, shipmentID)
	if err != nil {
		s.log.Error().Err(err).Str("shipment_id", shipmentID).Msg("failed to find events")
		return nil, fmt.Errorf("finding events: %w", err)
	}

	s.log.Debug().
		Str("shipment_id", shipmentID).
		Int("event_count", len(events)).
		Msg("shipment history retrieved")

	return events, nil
}
