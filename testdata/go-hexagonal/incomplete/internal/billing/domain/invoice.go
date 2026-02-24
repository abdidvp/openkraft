package domain

import "time"

// Invoice represents a billing invoice.
type Invoice struct {
	ID        string
	OrderID   string
	Amount    float64
	Currency  string
	Status    string
	IssuedAt  time.Time
}
