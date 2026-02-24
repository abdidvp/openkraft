package domain

type Payment struct {
	ID       string
	Amount   float64
	Currency string
	Status   string
}
