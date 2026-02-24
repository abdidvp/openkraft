package repository

import (
	"errors"
	"sync"

	"example.com/incomplete/internal/orders/domain"
)

// querier abstracts the database query interface.
type querier interface {
	Get(id string) (interface{}, bool)
	Set(id string, value interface{})
}

// memoryStore is a simple in-memory implementation of querier.
type memoryStore struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

func newMemoryStore() *memoryStore {
	return &memoryStore{data: make(map[string]interface{})}
}

func (m *memoryStore) Get(id string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[id]
	return v, ok
}

func (m *memoryStore) Set(id string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[id] = value
}

// InMemoryOrderRepository is an in-memory implementation of the order repository.
type InMemoryOrderRepository struct {
	q querier
}

// NewInMemoryOrderRepository creates a new InMemoryOrderRepository.
func NewInMemoryOrderRepository() *InMemoryOrderRepository {
	return &InMemoryOrderRepository{q: newMemoryStore()}
}

func (r *InMemoryOrderRepository) getQuerier() querier {
	return r.q
}

// Save persists an order.
func (r *InMemoryOrderRepository) Save(order *domain.Order) error {
	if order.ID == "" {
		return errors.New("order ID is required")
	}
	r.getQuerier().Set(order.ID, order)
	return nil
}

// FindByID retrieves an order by ID.
func (r *InMemoryOrderRepository) FindByID(id string) (*domain.Order, error) {
	v, ok := r.getQuerier().Get(id)
	if !ok {
		return nil, errors.New("order not found")
	}
	order, ok := v.(*domain.Order)
	if !ok {
		return nil, errors.New("invalid order data")
	}
	return order, nil
}

// FindByCustomerID retrieves all orders for a customer.
func (r *InMemoryOrderRepository) FindByCustomerID(_ string) ([]*domain.Order, error) {
	return nil, nil
}
