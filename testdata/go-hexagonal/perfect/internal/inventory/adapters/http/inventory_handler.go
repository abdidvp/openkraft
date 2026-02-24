package http

import (
	"encoding/json"
	"net/http"

	"example.com/perfect/internal/inventory/application"
)

type InventoryHandler struct {
	service *application.InventoryService
}

func NewInventoryHandler(service *application.InventoryService) *InventoryHandler {
	return &InventoryHandler{service: service}
}

func (h *InventoryHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
