package application

import "example.com/perfect/internal/inventory/domain"

type ProductRepository interface {
	Create(product *domain.Product) error
	GetByID(id string) (*domain.Product, error)
	List() ([]*domain.Product, error)
}
