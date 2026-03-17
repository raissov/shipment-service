package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/raissov/shipment-service/internal/domain"
)

// StatusEventRepository is a PostgreSQL implementation of domain.StatusEventRepository.
type StatusEventRepository struct {
	pool *pgxpool.Pool
}

// NewStatusEventRepository creates a new PostgreSQL status event repository.
func NewStatusEventRepository(pool *pgxpool.Pool) *StatusEventRepository {
	return &StatusEventRepository{pool: pool}
}

func (r *StatusEventRepository) Save(ctx context.Context, e *domain.StatusEvent) error {
	db := getDB(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO status_events (id, shipment_id, status, comment, occurred_at)
		VALUES ($1, $2, $3, $4, $5)`,
		e.ID, e.ShipmentID, string(e.Status), e.Comment, e.OccurredAt,
	)
	return err
}

func (r *StatusEventRepository) FindByShipmentID(ctx context.Context, shipmentID string) ([]*domain.StatusEvent, error) {
	db := getDB(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id, shipment_id, status, comment, occurred_at
		FROM status_events
		WHERE shipment_id = $1
		ORDER BY occurred_at ASC`, shipmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*domain.StatusEvent
	for rows.Next() {
		e := &domain.StatusEvent{}
		var status string
		if err := rows.Scan(&e.ID, &e.ShipmentID, &status, &e.Comment, &e.OccurredAt); err != nil {
			return nil, err
		}
		e.Status = domain.Status(status)
		events = append(events, e)
	}
	return events, rows.Err()
}
