package domain

import (
	"errors"
	"time"
)

type TaxRule struct {
	ID        string
	Name      string
	Rate      float64
	Country   string
	ValidFrom time.Time
	ValidTo   time.Time
	CreatedAt time.Time
}

func NewTaxRule(name string, rate float64, country string) (*TaxRule, error) {
	rule := &TaxRule{
		Name:      name,
		Rate:      rate,
		Country:   country,
		CreatedAt: time.Now(),
	}
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	return rule, nil
}

func (r *TaxRule) Validate() error {
	if r.Name == "" {
		return errors.New("tax rule name is required")
	}
	if r.Rate < 0 || r.Rate > 100 {
		return errors.New("tax rate must be between 0 and 100")
	}
	if r.Country == "" {
		return errors.New("country is required")
	}
	return nil
}
