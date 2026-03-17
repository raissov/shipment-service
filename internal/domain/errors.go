package domain

import "errors"

var (
	ErrShipmentNotFound  = errors.New("shipment not found")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrInvalidStatus     = errors.New("invalid status")
	ErrDuplicateStatus   = errors.New("shipment already has this status")
)
