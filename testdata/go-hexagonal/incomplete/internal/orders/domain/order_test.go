package domain

import "testing"

func TestNewOrder(t *testing.T) {
	items := []OrderItem{
		{ProductID: "prod-1", Quantity: 2, Price: 9.99},
	}
	order := NewOrder("cust-1", items)

	if order.CustomerID != "cust-1" {
		t.Errorf("expected customer ID cust-1, got %s", order.CustomerID)
	}
	if order.Status != "pending" {
		t.Errorf("expected status pending, got %s", order.Status)
	}
	if len(order.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(order.Items))
	}
}

func TestOrder_Validate(t *testing.T) {
	tests := []struct {
		name    string
		order   *Order
		wantErr bool
	}{
		{
			name:    "valid order",
			order:   NewOrder("cust-1", []OrderItem{{ProductID: "p1", Quantity: 1, Price: 10.0}}),
			wantErr: false,
		},
		{
			name:    "missing customer ID",
			order:   NewOrder("", []OrderItem{{ProductID: "p1", Quantity: 1, Price: 10.0}}),
			wantErr: true,
		},
		{
			name:    "no items",
			order:   NewOrder("cust-1", nil),
			wantErr: true,
		},
		{
			name:    "zero quantity",
			order:   NewOrder("cust-1", []OrderItem{{ProductID: "p1", Quantity: 0, Price: 10.0}}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.order.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
