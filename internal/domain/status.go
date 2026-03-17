package domain

import "fmt"

// Status represents a shipment status in the domain.
type Status string

const (
	StatusPending   Status = "pending"
	StatusPickedUp  Status = "picked_up"
	StatusInTransit Status = "in_transit"
	StatusDelivered Status = "delivered"
	StatusCancelled Status = "cancelled"
)

// validTransitions defines the allowed status transitions.
// Key is the current status, value is the set of statuses it can transition to.
var validTransitions = map[Status]map[Status]bool{
	StatusPending: {
		StatusPickedUp:  true,
		StatusCancelled: true,
	},
	StatusPickedUp: {
		StatusInTransit: true,
		StatusCancelled: true,
	},
	StatusInTransit: {
		StatusDelivered: true,
		StatusCancelled: true,
	},
	// Delivered and Cancelled are terminal states — no further transitions allowed.
	StatusDelivered: {},
	StatusCancelled: {},
}

// CanTransitionTo checks whether a transition from the current status to the target is valid.
func (s Status) CanTransitionTo(target Status) bool {
	targets, ok := validTransitions[s]
	if !ok {
		return false
	}
	return targets[target]
}

// ValidateTransition returns an error if the transition is not allowed.
func (s Status) ValidateTransition(target Status) error {
	if !s.CanTransitionTo(target) {
		return fmt.Errorf("%w: cannot transition from %q to %q", ErrInvalidTransition, s, target)
	}
	return nil
}

// IsValid checks whether the status value is a known status.
func (s Status) IsValid() bool {
	_, ok := validTransitions[s]
	return ok
}
