CREATE TABLE IF NOT EXISTS shipments (
    id UUID PRIMARY KEY,
    reference_number VARCHAR(255) NOT NULL,
    origin VARCHAR(255) NOT NULL,
    destination VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    driver_name VARCHAR(255) NOT NULL DEFAULT '',
    driver_phone VARCHAR(50) NOT NULL DEFAULT '',
    unit_number VARCHAR(100) NOT NULL DEFAULT '',
    unit_type VARCHAR(100) NOT NULL DEFAULT '',
    shipment_amount_cents BIGINT NOT NULL DEFAULT 0,
    driver_revenue_cents BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_shipments_reference_number ON shipments (reference_number);
CREATE INDEX idx_shipments_status ON shipments (status);
