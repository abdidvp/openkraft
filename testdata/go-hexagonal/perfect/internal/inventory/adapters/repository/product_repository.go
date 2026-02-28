package repository

import (
	"fmt"
	"sort"
	"sync"

	"example.com/perfect/internal/inventory/domain"
)

// PostgresProductRepository stores products using an indexed map for efficient
// lookup by both ID and SKU. Unlike the tax repository (simple key-value), the
// product repository supports secondary indexes and sorted listing.
type PostgresProductRepository struct {
	mu       sync.RWMutex
	products map[string]*domain.Product
	bySKU    map[string]string // SKU → ID reverse index
}

func NewPostgresProductRepository() *PostgresProductRepository {
	return &PostgresProductRepository{
		products: make(map[string]*domain.Product),
		bySKU:    make(map[string]string),
	}
}

func (r *PostgresProductRepository) Create(product *domain.Product) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.products[product.ID]; exists {
		return fmt.Errorf("product %s already exists", product.ID)
	}
	r.products[product.ID] = product
	if product.SKU != "" {
		r.bySKU[product.SKU] = product.ID
	}
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

func (r *PostgresProductRepository) GetBySKU(sku string) (*domain.Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.bySKU[sku]
	if !ok {
		return nil, domain.ErrProductNotFound
	}
	return r.products[id], nil
}

func (r *PostgresProductRepository) List() ([]*domain.Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.Product, 0, len(r.products))
	for _, p := range r.products {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func (r *PostgresProductRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.products[id]
	if !ok {
		return domain.ErrProductNotFound
	}
	delete(r.bySKU, p.SKU)
	delete(r.products, id)
	return nil
}
