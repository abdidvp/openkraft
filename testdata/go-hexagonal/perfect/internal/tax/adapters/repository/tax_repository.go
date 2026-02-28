package repository

import (
	"example.com/perfect/internal/tax/domain"
)

// PostgresTaxRuleRepository stores tax rules in an append-only slice with
// country-based indexing for efficient lookups by jurisdiction.
type PostgresTaxRuleRepository struct {
	entries   []*domain.TaxRule
	byCountry map[string][]*domain.TaxRule
}

func NewPostgresTaxRuleRepository() *PostgresTaxRuleRepository {
	return &PostgresTaxRuleRepository{
		byCountry: make(map[string][]*domain.TaxRule),
	}
}

func (r *PostgresTaxRuleRepository) Create(rule *domain.TaxRule) error {
	r.entries = append(r.entries, rule)
	r.byCountry[rule.Country] = append(r.byCountry[rule.Country], rule)
	return nil
}

func (r *PostgresTaxRuleRepository) GetByID(id string) (*domain.TaxRule, error) {
	for _, entry := range r.entries {
		if entry.ID == id {
			return entry, nil
		}
	}
	return nil, domain.ErrTaxRuleNotFound
}

func (r *PostgresTaxRuleRepository) ListByCountry(country string) ([]*domain.TaxRule, error) {
	rules, ok := r.byCountry[country]
	if !ok {
		return nil, nil
	}
	out := make([]*domain.TaxRule, len(rules))
	copy(out, rules)
	return out, nil
}

func (r *PostgresTaxRuleRepository) List() ([]*domain.TaxRule, error) {
	out := make([]*domain.TaxRule, len(r.entries))
	copy(out, r.entries)
	return out, nil
}

func getQuerier() string {
	return "postgres"
}
