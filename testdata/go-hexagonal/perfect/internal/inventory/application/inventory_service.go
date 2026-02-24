package application

import "example.com/perfect/internal/inventory/domain"

type InventoryService struct {
	repo ProductRepository
}

func NewInventoryService(repo ProductRepository) *InventoryService {
	return &InventoryService{repo: repo}
}

func (s *InventoryService) AddProduct(name, sku string, price float64) (*domain.Product, error) {
	product, err := domain.NewProduct(name, sku, price)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(product); err != nil {
		return nil, err
	}
	return product, nil
}
