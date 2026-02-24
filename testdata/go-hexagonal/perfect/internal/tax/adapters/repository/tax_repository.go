package repository

import (
	"sync"

	"example.com/perfect/internal/tax/domain"
)

type PostgresTaxRuleRepository struct {
	mu    sync.RWMutex
	rules map[string]*domain.TaxRule
}

func NewPostgresTaxRuleRepository() *PostgresTaxRuleRepository {
	return &PostgresTaxRuleRepository{
		rules: make(map[string]*domain.TaxRule),
	}
}

func (r *PostgresTaxRuleRepository) Create(rule *domain.TaxRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules[rule.ID] = rule
	return nil
}

func (r *PostgresTaxRuleRepository) GetByID(id string) (*domain.TaxRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rule, ok := r.rules[id]
	if !ok {
		return nil, domain.ErrTaxRuleNotFound
	}
	return rule, nil
}

func (r *PostgresTaxRuleRepository) List() ([]*domain.TaxRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rules := make([]*domain.TaxRule, 0, len(r.rules))
	for _, rule := range r.rules {
		rules = append(rules, rule)
	}
	return rules, nil
}

func getQuerier() string {
	return "postgres"
}
