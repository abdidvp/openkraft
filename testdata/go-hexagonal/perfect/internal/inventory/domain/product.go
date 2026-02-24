package domain

import "errors"

type Product struct {
	ID       string
	Name     string
	SKU      string
	Price    float64
	Quantity int
}

func NewProduct(name, sku string, price float64) (*Product, error) {
	p := &Product{
		Name:  name,
		SKU:   sku,
		Price: price,
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Product) Validate() error {
	if p.Name == "" {
		return errors.New("product name is required")
	}
	if p.SKU == "" {
		return errors.New("product SKU is required")
	}
	if p.Price < 0 {
		return errors.New("price must be non-negative")
	}
	return nil
}
