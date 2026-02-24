package domain_test

import (
	"testing"

	"example.com/perfect/internal/tax/domain"
)

func TestNewTaxRule(t *testing.T) {
	rule, err := domain.NewTaxRule("VAT", 21.0, "ES")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.Name != "VAT" {
		t.Errorf("expected name VAT, got %s", rule.Name)
	}
}

func TestTaxRule_Validate(t *testing.T) {
	tests := []struct {
		name    string
		rule    domain.TaxRule
		wantErr bool
	}{
		{"valid", domain.TaxRule{Name: "VAT", Rate: 21, Country: "ES"}, false},
		{"empty name", domain.TaxRule{Rate: 21, Country: "ES"}, true},
		{"negative rate", domain.TaxRule{Name: "VAT", Rate: -1, Country: "ES"}, true},
		{"empty country", domain.TaxRule{Name: "VAT", Rate: 21}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
