package application

import "example.com/incomplete/internal/shipping/domain"

// ShippingService handles shipment business logic.
type ShippingService struct{}

// NewShippingService creates a new ShippingService.
func NewShippingService() *ShippingService {
	return &ShippingService{}
}

// CreateShipment creates and validates a new shipment.
func (s *ShippingService) CreateShipment(orderID string, address string) (*domain.Shipment, error) {
	shipment := domain.NewShipment(orderID, address)
	if err := shipment.Validate(); err != nil {
		return nil, err
	}
	return shipment, nil
}
