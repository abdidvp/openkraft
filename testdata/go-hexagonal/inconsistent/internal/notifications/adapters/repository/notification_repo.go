package repository

import (
	"errors"
	"sync"

	"example.com/inconsistent/internal/notifications/domain"
)

// InMemoryNotificationRepo is an in-memory notification repository.
type InMemoryNotificationRepo struct {
	mu   sync.RWMutex
	data map[string]*domain.Notification
}

// NewInMemoryNotificationRepo creates a new InMemoryNotificationRepo.
func NewInMemoryNotificationRepo() *InMemoryNotificationRepo {
	return &InMemoryNotificationRepo{
		data: make(map[string]*domain.Notification),
	}
}

// Save persists a notification.
func (r *InMemoryNotificationRepo) Save(n *domain.Notification) error {
	if n.ID == "" {
		return errors.New("notification ID is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[n.ID] = n
	return nil
}

// FindByID retrieves a notification by ID.
func (r *InMemoryNotificationRepo) FindByID(id string) (*domain.Notification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n, ok := r.data[id]
	if !ok {
		return nil, errors.New("notification not found")
	}
	return n, nil
}
