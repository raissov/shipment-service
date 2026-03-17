CREATE TABLE IF NOT EXISTS status_events (
    id UUID PRIMARY KEY,
    shipment_id UUID NOT NULL REFERENCES shipments(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_status_events_shipment_id ON status_events (shipment_id);
CREATE INDEX idx_status_events_occurred_at ON status_events (shipment_id, occurred_at);
