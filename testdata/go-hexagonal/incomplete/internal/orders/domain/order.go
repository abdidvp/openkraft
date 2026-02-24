package domain

import (
	"errors"
	"time"
)

// Order represents a customer order in the system.
type Order struct {
	ID         string
	CustomerID string
	Items      []OrderItem
	Status     string
	CreatedAt  time.Time
}

// OrderItem represents a single item within an order.
type OrderItem struct {
	ProductID string
	Quantity  int
	Price     float64
}

// NewOrder creates a new Order with the given customer ID and items.
func NewOrder(customerID string, items []OrderItem) *Order {
	return &Order{
		CustomerID: customerID,
		Items:      items,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
}

// Validate checks that the order has valid data.
func (o *Order) Validate() error {
	if o.CustomerID == "" {
		return errors.New("customer ID is required")
	}
	if len(o.Items) == 0 {
		return errors.New("order must have at least one item")
	}
	for _, item := range o.Items {
		if item.Quantity <= 0 {
			return errors.New("item quantity must be positive")
		}
		if item.Price < 0 {
			return errors.New("item price must not be negative")
		}
	}
	return nil
}
