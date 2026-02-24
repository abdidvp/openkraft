package repository

import (
	"sync"

	"example.com/perfect/internal/inventory/domain"
)

type PostgresProductRepository struct {
	mu       sync.RWMutex
	products map[string]*domain.Product
}

func NewPostgresProductRepository() *PostgresProductRepository {
	return &PostgresProductRepository{
		products: make(map[string]*domain.Product),
	}
}

func (r *PostgresProductRepository) Create(product *domain.Product) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.products[product.ID] = product
	return nil
}

func (r *PostgresProductRepository) GetByID(id string) (*domain.Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	product, ok := r.products[id]
	if !ok {
		return nil, domain.ErrProductNotFound
	}
	return product, nil
}

func (r *PostgresProductRepository) List() ([]*domain.Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	products := make([]*domain.Product, 0, len(r.products))
	for _, p := range r.products {
		products = append(products, p)
	}
	return products, nil
}

func getQuerier() string {
	return "postgres"
}
