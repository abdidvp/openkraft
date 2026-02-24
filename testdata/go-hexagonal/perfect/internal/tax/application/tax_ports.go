package application

import "example.com/perfect/internal/tax/domain"

type TaxRuleRepository interface {
	Create(rule *domain.TaxRule) error
	GetByID(id string) (*domain.TaxRule, error)
	List() ([]*domain.TaxRule, error)
}
