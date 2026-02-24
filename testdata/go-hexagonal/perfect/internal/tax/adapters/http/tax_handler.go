package http

import (
	"encoding/json"
	"net/http"

	"example.com/perfect/internal/tax/application"
)

type TaxHandler struct {
	service *application.TaxService
}

func NewTaxHandler(service *application.TaxService) *TaxHandler {
	return &TaxHandler{service: service}
}

func (h *TaxHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.service.ListRules()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}
