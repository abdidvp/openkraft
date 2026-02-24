package application

import "example.com/perfect/internal/tax/domain"

type TaxService struct {
	repo TaxRuleRepository
}

func NewTaxService(repo TaxRuleRepository) *TaxService {
	return &TaxService{repo: repo}
}

func (s *TaxService) CreateRule(name string, rate float64, country string) (*domain.TaxRule, error) {
	rule, err := domain.NewTaxRule(name, rate, country)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *TaxService) GetRule(id string) (*domain.TaxRule, error) {
	return s.repo.GetByID(id)
}

func (s *TaxService) ListRules() ([]*domain.TaxRule, error) {
	return s.repo.List()
}
