package domain

import (
	"errors"
	"time"
)

// Shipment represents a shipment for an order.
type Shipment struct {
	ID        string
	OrderID   string
	Address   string
	Status    string
	CreatedAt time.Time
}

// NewShipment creates a new Shipment for the given order.
func NewShipment(orderID string, address string) *Shipment {
	return &Shipment{
		OrderID:   orderID,
		Address:   address,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
}

// Validate checks that the shipment has valid data.
func (s *Shipment) Validate() error {
	if s.OrderID == "" {
		return errors.New("order ID is required")
	}
	if s.Address == "" {
		return errors.New("address is required")
	}
	return nil
}
