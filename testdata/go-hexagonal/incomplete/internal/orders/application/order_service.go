package application

import "example.com/incomplete/internal/orders/domain"

// OrderService handles order business logic.
type OrderService struct {
	repo OrderRepository
}

// NewOrderService creates a new OrderService with the given repository.
func NewOrderService(repo OrderRepository) *OrderService {
	return &OrderService{repo: repo}
}

// CreateOrder validates and persists a new order.
func (s *OrderService) CreateOrder(customerID string, items []domain.OrderItem) (*domain.Order, error) {
	order := domain.NewOrder(customerID, items)
	if err := order.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Save(order); err != nil {
		return nil, err
	}
	return order, nil
}

// GetOrder retrieves an order by its ID.
func (s *OrderService) GetOrder(id string) (*domain.Order, error) {
	return s.repo.FindByID(id)
}
