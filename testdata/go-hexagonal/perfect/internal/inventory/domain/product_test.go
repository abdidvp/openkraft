package domain_test

import (
	"testing"

	"example.com/perfect/internal/inventory/domain"
)

func TestNewProduct(t *testing.T) {
	p, err := domain.NewProduct("Widget", "WDG-001", 9.99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "Widget" {
		t.Errorf("expected Widget, got %s", p.Name)
	}
}

func TestProduct_Validate(t *testing.T) {
	tests := []struct {
		name    string
		product domain.Product
		wantErr bool
	}{
		{"valid", domain.Product{Name: "W", SKU: "S", Price: 1}, false},
		{"empty name", domain.Product{SKU: "S", Price: 1}, true},
		{"empty sku", domain.Product{Name: "W", Price: 1}, true},
		{"negative price", domain.Product{Name: "W", SKU: "S", Price: -1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.product.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
