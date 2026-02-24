package http

import (
	"encoding/json"
	"net/http"

	"example.com/incomplete/internal/orders/application"
	"example.com/incomplete/internal/orders/domain"
)

// OrderHandler handles HTTP requests for orders.
type OrderHandler struct {
	service *application.OrderService
}

// NewOrderHandler creates a new OrderHandler.
func NewOrderHandler(service *application.OrderService) *OrderHandler {
	return &OrderHandler{service: service}
}

// CreateOrderRequest represents the request body for creating an order.
type CreateOrderRequest struct {
	CustomerID string             `json:"customer_id"`
	Items      []domain.OrderItem `json:"items"`
}

// HandleCreateOrder handles POST requests to create an order.
func (h *OrderHandler) HandleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	order, err := h.service.CreateOrder(req.CustomerID, req.Items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}
