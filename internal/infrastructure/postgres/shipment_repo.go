package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/raissov/shipment-service/internal/domain"
)

// ShipmentRepository is a PostgreSQL implementation of domain.ShipmentRepository.
type ShipmentRepository struct {
	pool *pgxpool.Pool
}

// NewShipmentRepository creates a new PostgreSQL shipment repository.
func NewShipmentRepository(pool *pgxpool.Pool) *ShipmentRepository {
	return &ShipmentRepository{pool: pool}
}

func (r *ShipmentRepository) Save(ctx context.Context, s *domain.Shipment) error {
	db := getDB(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO shipments (
			id, reference_number, origin, destination, status,
			driver_name, driver_phone, unit_number, unit_type,
			shipment_amount_cents, driver_revenue_cents,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		s.ID, s.ReferenceNumber, s.Origin, s.Destination, string(s.Status),
		s.Driver.Name, s.Driver.Phone, s.Unit.UnitNumber, s.Unit.UnitType,
		s.ShipmentAmountCents, s.DriverRevenueCents,
		s.CreatedAt, s.UpdatedAt,
	)
	return err
}

func (r *ShipmentRepository) FindByID(ctx context.Context, id string) (*domain.Shipment, error) {
	db := getDB(ctx, r.pool)
	row := db.QueryRow(ctx, `
		SELECT id, reference_number, origin, destination, status,
		       driver_name, driver_phone, unit_number, unit_type,
		       shipment_amount_cents, driver_revenue_cents,
		       created_at, updated_at
		FROM shipments WHERE id = $1`, id)

	return scanShipment(row)
}

func (r *ShipmentRepository) Update(ctx context.Context, s *domain.Shipment) error {
	db := getDB(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		UPDATE shipments
		SET status = $1, updated_at = $2
		WHERE id = $3`,
		string(s.Status), s.UpdatedAt, s.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrShipmentNotFound
	}
	return nil
}

func (r *ShipmentRepository) List(ctx context.Context, limit, offset int) ([]*domain.Shipment, int, error) {
	db := getDB(ctx, r.pool)

	var total int
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM shipments`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(ctx, `
		SELECT id, reference_number, origin, destination, status,
		       driver_name, driver_phone, unit_number, unit_type,
		       shipment_amount_cents, driver_revenue_cents,
		       created_at, updated_at
		FROM shipments
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var shipments []*domain.Shipment
	for rows.Next() {
		s, err := scanShipment(rows)
		if err != nil {
			return nil, 0, err
		}
		shipments = append(shipments, s)
	}
	return shipments, total, rows.Err()
}

// scannable is the common interface for pgx.Row and pgx.Rows.
type scannable interface {
	Scan(dest ...any) error
}

func scanShipment(row scannable) (*domain.Shipment, error) {
	s := &domain.Shipment{}
	var status string
	err := row.Scan(
		&s.ID, &s.ReferenceNumber, &s.Origin, &s.Destination, &status,
		&s.Driver.Name, &s.Driver.Phone, &s.Unit.UnitNumber, &s.Unit.UnitType,
		&s.ShipmentAmountCents, &s.DriverRevenueCents,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrShipmentNotFound
		}
		return nil, err
	}
	s.Status = domain.Status(status)
	return s, nil
}
