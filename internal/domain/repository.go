package domain

import "context"

// ShipmentRepository defines the persistence interface for shipments.
// This interface belongs to the domain layer — implementations live in infrastructure.
type ShipmentRepository interface {
	Save(ctx context.Context, shipment *Shipment) error
	FindByID(ctx context.Context, id string) (*Shipment, error)
	Update(ctx context.Context, shipment *Shipment) error
	List(ctx context.Context, limit, offset int) ([]*Shipment, int, error)
}

// StatusEventRepository defines the persistence interface for status events.
type StatusEventRepository interface {
	Save(ctx context.Context, event *StatusEvent) error
	FindByShipmentID(ctx context.Context, shipmentID string) ([]*StatusEvent, error)
}

// TxManager abstracts database transaction management.
type TxManager interface {
	// WithTx executes fn inside a database transaction.
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
