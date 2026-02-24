package application

import "example.com/incomplete/internal/orders/domain"

// OrderRepository defines the interface for order persistence.
type OrderRepository interface {
	Save(order *domain.Order) error
	FindByID(id string) (*domain.Order, error)
	FindByCustomerID(customerID string) ([]*domain.Order, error)
}
